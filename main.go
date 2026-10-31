// Tesseract on the unmodified image, followed by MRZ parsing and validation.
// POST http://localhost:8082/mrz with multipart file field "image".
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"passport-tooles/mrz"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/otiai10/gosseract/v2"
)

const maxUploadBytes = 20 << 20

var sequence atomic.Int64

func init() { sequence.Store(time.Now().UnixMilli()) }

// recognize sends the uploaded bytes directly to Tesseract, once.
func recognize(img []byte) (string, error) {
	client := gosseract.NewClient()
	defer client.Close()
	if prefix := os.Getenv("MRZ_TESSDATA_PREFIX"); prefix != "" {
		if err := client.SetTessdataPrefix(prefix); err != nil {
			return "", err
		}
	}
	if language := os.Getenv("MRZ_TESSERACT_LANG"); language != "" {
		if err := client.SetLanguage(language); err != nil {
			return "", err
		}
	}
	if err := client.SetPageSegMode(gosseract.PSM_SINGLE_BLOCK); err != nil {
		return "", err
	}
	if err := client.SetImageFromBytes(img); err != nil {
		return "", err
	}
	return client.Text()
}

func writeResult(w http.ResponseWriter, status int, result mrz.Result) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result); err != nil {
		log.Printf("encode OCR response: %v", err)
	}
}

func newHandler(ocr func([]byte) (string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID, jobID := sequence.Add(1), sequence.Add(1)
		fail := func(status int, message string) {
			writeResult(w, status, mrz.Failure(requestID, jobID, message))
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			fail(http.StatusMethodNotAllowed, "use POST with multipart file field 'image'")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
		err := r.ParseMultipartForm(maxUploadBytes)
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		if err != nil {
			fail(http.StatusBadRequest, "invalid multipart upload (maximum 20 MiB)")
			return
		}
		for _, field := range []struct {
			name string
			id   *int64
		}{{"requestId", &requestID}, {"jobId", &jobID}} {
			if value := r.FormValue(field.name); value != "" {
				id, err := strconv.ParseInt(value, 10, 64)
				if err != nil || id <= 0 {
					fail(http.StatusBadRequest, fmt.Sprintf("%s must be a positive integer", field.name))
					return
				}
				*field.id = id
			}
		}
		file, _, err := r.FormFile("image")
		if err != nil {
			fail(http.StatusBadRequest, "missing form-data file field 'image'")
			return
		}
		defer file.Close()
		img, err := io.ReadAll(file)
		if err != nil || len(img) == 0 {
			fail(http.StatusBadRequest, "could not read a nonempty image")
			return
		}
		text, err := ocr(img)
		fmt.Printf("[tesseract_raw requestId=%d jobId=%d] raw OCR text before parsing:\n%s\n", requestID, jobID, text)
		if err != nil {
			log.Printf("tesseract OCR failed: %v", err)
			fail(http.StatusBadGateway, "tesseract could not recognize the image")
			return
		}
		if strings.TrimSpace(text) == "" {
			fail(http.StatusUnprocessableEntity, "tesseract produced no text")
			return
		}
		result := mrz.Response(requestID, jobID, strings.Split(text, "\n"))
		status := http.StatusOK
		if result.Status == mrz.StatusFailed {
			status = http.StatusUnprocessableEntity
		}
		writeResult(w, status, result)
	}
}

func handler(w http.ResponseWriter, r *http.Request) { newHandler(recognize)(w, r) }

func main() {
	http.HandleFunc("/mrz", handler)
	log.Println("tesseract_raw listening on :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
