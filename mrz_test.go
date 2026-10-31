package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"passport-tooles/mrz"
	"strings"
	"testing"
)

const sampleText = "P<IRNIRANI<<FATEMEH<<<<<<<<<<<<<<<<<<<<<<<<<\nS000028129IRN7902114F1910094<<<<<<<<<<<<<<06"

func uploadRequest(t *testing.T, image []byte, fields map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for name, value := range fields {
		if err := form.WriteField(name, value); err != nil {
			t.Fatal(err)
		}
	}
	if image != nil {
		part, err := form.CreateFormFile("image", "passport.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(image); err != nil {
			t.Fatal(err)
		}
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/mrz", &body)
	r.Header.Set("Content-Type", form.FormDataContentType())
	return r
}

func TestUploadReturnsPassport(t *testing.T) {
	img := []byte("original image bytes")
	calls := 0
	h := newHandler(func(got []byte) (string, error) {
		calls++
		if !bytes.Equal(got, img) {
			t.Fatalf("image changed: %q", got)
		}
		return sampleText, nil
	})
	w := httptest.NewRecorder()
	h(w, uploadRequest(t, img, map[string]string{"requestId": "109394", "jobId": "109395"}))
	if w.Code != http.StatusOK {
		t.Fatalf("%d: %s", w.Code, w.Body.String())
	}
	if calls != 1 {
		t.Fatalf("OCR called %d times", calls)
	}
	var result mrz.Result
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.RequestID != 109394 || result.JobID != 109395 || result.Status != 2 || result.Error != nil || result.GeneralResult != nil {
		t.Fatalf("unexpected envelope: %+v", result)
	}
	p := result.PassportResult
	if p == nil || p.FirstName != "FATEMEH" || p.LastName != "IRANI" || p.FullNameLatin != "FATEMEH IRANI" || p.PassportNumber != "S00002812" || p.BirthDate != "1979-02-11" || p.ExpiryDate != "2019-10-09" || p.MRZRawText != sampleText {
		t.Fatalf("unexpected passport: %+v", p)
	}
	if !p.AllChecksumsValid {
		t.Fatal("sample passport checksums failed")
	}
	for i, check := range []*mrz.Checksum{p.PassportNumberChecksum, p.BirthDateChecksum, p.ExpiryDateChecksum, p.PersonalNumberChecksum, p.CompositeChecksum} {
		want := []int{9, 4, 4, 0, 6}[i]
		if check == nil || check.Expected == nil || *check.Expected != want || check.Calculated != want || !check.IsValid {
			t.Fatalf("sample checksum %d: %+v", i, check)
		}
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 6 || string(raw["generalResult"]) != "null" || string(raw["error"]) != "null" {
		t.Fatalf("unexpected JSON: %s", w.Body.String())
	}
}

func TestUploadErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		image  []byte
		text   string
		err    error
		status int
	}{
		{"missing image", nil, "", nil, 400},
		{"empty image", []byte{}, "", nil, 400},
		{"OCR failure", []byte("image"), "", errors.New("OCR unavailable"), 502},
		{"no text", []byte("image"), " \n", nil, 422},
		{"no MRZ", []byte("image"), "unrecognized text", nil, 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			newHandler(func([]byte) (string, error) { return tc.text, tc.err })(w, uploadRequest(t, tc.image, nil))
			var result mrz.Result
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.status || result.Status != mrz.StatusFailed || result.Error == nil || result.PassportResult != nil || result.RequestID <= 0 || result.JobID <= 0 {
				t.Fatalf("%d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestRequestValidation(t *testing.T) {
	for _, r := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/mrz", nil),
		httptest.NewRequest(http.MethodPost, "/mrz", strings.NewReader("not multipart")),
		uploadRequest(t, []byte("image"), map[string]string{"requestId": "invalid"}),
		uploadRequest(t, []byte("image"), map[string]string{"jobId": "-1"}),
	} {
		w := httptest.NewRecorder()
		newHandler(func([]byte) (string, error) { t.Fatal("invalid request reached OCR"); return "", nil })(w, r)
		if w.Code < 400 || w.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("%d: %s", w.Code, w.Body.String())
		}
	}
}

// Opt in with MRZ_TEST_IMAGE_PATH pointing to the supplied regression image.
func TestRecognizePassportImage(t *testing.T) {
	path := os.Getenv("MRZ_TEST_IMAGE_PATH")
	if path == "" {
		t.Skip("set MRZ_TEST_IMAGE_PATH to run the real Tesseract regression")
	}
	img, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	newHandler(recognize)(w, uploadRequest(t, img, nil))
	var result mrz.Result
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	p := result.PassportResult
	if w.Code != http.StatusOK || p == nil || !p.AllChecksumsValid || p.FirstName != "FATEMEH" || p.LastName != "IRANI" || p.PassportNumber != "S00002812" || p.Gender != "F" || p.BirthDate != "1979-02-11" || p.ExpiryDate != "2019-10-09" {
		t.Fatalf("unexpected real OCR response: %s", w.Body.String())
	}
	t.Logf("real OCR response: %s", w.Body.String())
}
