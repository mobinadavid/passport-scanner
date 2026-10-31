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
