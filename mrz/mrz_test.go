package mrz

import "testing"

func TestParsePadsDroppedTrailingFiller(t *testing.T) {
	lines := []string{
		"P<UTOERIKSSON<<ANNA<MARIA<<<<<<<<<<<<<<<<<<",
		"L898902C36UTO7408122F1204159ZE184226B<<<<<10",
	}
	d, err := Parse(lines)
	if err != nil {
		t.Fatal(err)
	}
	if d.Surname != "ERIKSSON" || d.GivenNames != "ANNA MARIA" {
		t.Fatalf("unexpected names: %q / %q", d.Surname, d.GivenNames)
	}
}

func TestParseTrimsSmallOCROverflow(t *testing.T) {
	lines := []string{
		"P<UTOERIKSSON<<ANNA<MARIA<<<<<<<<<<<<<<<<<<<<<",
		"L898902C36UTO7408122F1204159ZE184226B<<<<<10",
	}
	if _, err := Parse(lines); err != nil {
		t.Fatal(err)
	}
}

// Regression for the supplied passport image, recognized once with PSM 6.
func TestPassportImageOCR(t *testing.T) {
	d, err := Parse([]string{
		"P<IRNIRANI<<FATEMEH<K<<<<< <<< <6 KK KEK KK KK KKK",
		"$000028129IRN7902114F1910094<<<<<<<<<<<<<<06",
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.GivenNames != "FATEMEH" || d.Surname != "IRANI" || d.DocNumber != "S00002812" || d.Nationality != "IRN" || d.Sex != "F" || d.Birth != "1979-02-11" || d.Expiry != "2019-10-09" || !d.Valid() {
		t.Fatalf("incorrect passport: %+v", d)
	}
}

func TestMissingLeadingCharacterDoesNotReturnShiftedFields(t *testing.T) {
	_, err := Parse([]string{
		"P<IRNIRANI<<FATEMEH<<<<<<<<<<<KKKKEKKKKKKKK<",
		"0000281291RN7902114FI910094<<<<<<<<<<<<<<06<",
	})
	if err == nil {
		t.Fatal("accepted shifted fields with invalid dates and gender")
	}
}

func TestNamePadding(t *testing.T) {
	for _, tc := range []struct{ raw, surname, given string }{
		{"IRANI<<FATEMEH<<<<<<<<<<<KKKKEKKKKKKKK<", "IRANI", "FATEMEH"},
		{"ERIKSSON<<ANNA<MARIA<<<<<<<<<<<<", "ERIKSSON", "ANNA MARIA"},
		{"DE<LA<CRUZ<<ANA<MARIA<<<<KKKK", "DE LA CRUZ", "ANA MARIA"},
	} {
		surname, given := splitName(tc.raw)
		if surname != tc.surname || given != tc.given {
			t.Fatalf("%q: %q / %q", tc.raw, surname, given)
		}
	}
}

func TestPassportNameLineCollapsedTrailingFillers(t *testing.T) {
	d, err := Parse([]string{
		"P<IRNIRANI<<FATEMEH<<<<<<<< KKK KKK KKK KKK KKK",
		"$000028129IRN7902114F1910094<<<<<<<<<<<<<<06",
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.GivenNames != "FATEMEH" || d.Surname != "IRANI" || !d.Valid() {
		t.Fatalf("incorrect passport from cropped OCR: %+v", d)
	}
}
