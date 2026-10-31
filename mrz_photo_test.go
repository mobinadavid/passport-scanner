package main

import (
	"os"
	"passport-tooles/mrz"
	"path/filepath"
	"strings"
	"testing"
)

func TestPhotographedPassports(t *testing.T) {
	paths := os.Getenv("MRZ_TEST_PHOTO_PATHS")
	if paths == "" {
		t.Skip("set MRZ_TEST_PHOTO_PATHS to test photographed passports")
	}
	for _, path := range filepath.SplitList(paths) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if dir := os.Getenv("MRZ_TEST_CROP_DIR"); dir != "" {
				crop, location, err := prepareMRZ(data)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("location: %+v", location)
				if err := os.WriteFile(filepath.Join(dir, filepath.Base(path)+".png"), crop, 0600); err != nil {
					t.Fatal(err)
				}
			}
			text, err := recognize(data)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("OCR:\n%s", text)
			d, err := mrz.Parse(strings.Split(text, "\n"))
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("passport: %+v", d.Passport())
			if !d.Valid() {
				t.Fatalf("checks failed: %+v", d.Checks)
			}
			// Names have no check digit: verify the supplied regression photos
			// separately so valid checksums cannot hide a misspelled surname.
			expected := map[string][2]string{
				"codex-clipboard-2662d5b3-25f2-4ee9-8e8e-1ec096f8e838.jpg": {"MOBINA", "T72762439"},
				"codex-clipboard-2bddd340-6cd7-4a33-b1fc-71232c525a5c.jpg": {"TINA", "T72761923"},
			}
			if want, ok := expected[filepath.Base(path)]; ok {
				p := d.Passport()
				if p.FirstName != want[0] || p.LastName != "DAVOODIMOGHADDAM" ||
					p.PassportNumber != want[1] || p.BirthDate != "2004-06-13" ||
					p.ExpiryDate != "2030-10-26" || p.Gender != "F" || p.Nationality != "IRN" {
					t.Fatalf("incorrect passport fields: %+v", p)
				}
			}
		})
	}
}
