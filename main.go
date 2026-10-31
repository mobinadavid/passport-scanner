// Tesseract on the raw image + MRZ parser/validator.
//
//	cd servers/tesseract_raw && go mod tidy && go run .
//
// Postman: POST http://localhost:8081/mrz  form-data, key "image" (File)
package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"passport-tooles/mrz"
	"strings"
	"time"

	"github.com/otiai10/gosseract/v2"
)

func handler(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing form-data file field 'image'", 400)
		return
	}
	defer file.Close()
	img, _ := io.ReadAll(file)

	start := time.Now()
	var text string
	var lastText string
	bestScore := -1
	for _, psm := range []gosseract.PageSegMode{gosseract.PSM_SINGLE_BLOCK, gosseract.PSM_SINGLE_LINE} {
		client := gosseract.NewClient()
		client.SetWhitelist("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789<")
		if prefix := os.Getenv("MRZ_TESSDATA_PREFIX"); prefix != "" {
			_ = client.SetTessdataPrefix(prefix)
		}
		if language := os.Getenv("MRZ_TESSERACT_LANG"); language != "" {
			_ = client.SetLanguage(language)
		}
		client.SetPageSegMode(psm)
		if err := client.SetImageFromBytes(img); err != nil {
			client.Close()
			continue
		}
		candidate, err := client.Text()
		client.Close()
		if err != nil {
			continue
		}
		lastText = candidate
		score := -1
		if d, e := mrz.Parse(strings.Split(candidate, "\n")); e == nil {
			passed, total := d.Score()
			score = passed*100 - (total - passed)
		}
		if score > bestScore {
			bestScore, text = score, candidate
		}
	}
	if text == "" {
		text = lastText
	}
	if text == "" {
		http.Error(w, "tesseract produced no text", 502)
		return
	}

	resp := mrz.Response("tesseract_raw", time.Since(start).Milliseconds(),
		strings.Split(text, "\n"), map[string]any{"raw_text": text})
	log.Printf("[tesseract_raw] valid=%v passport=%+v", resp["valid"], resp["passport"])
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	http.HandleFunc("/mrz", handler)
	log.Println("tesseract_raw listening on :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
