package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func sampleMRZ() []string {
	return []string{
		"P<UTOERIKSSON<<ANNA<MARIA<<<<<<<<<<<<<<<<<<<",
		"L898902C36UTO7408122F1204159ZE184226B<<<<<10",
	}
}

func TestParseTD3(t *testing.T) {
	p, err := ParseTD3(sampleMRZ())
	if err != nil {
		t.Fatal(err)
	}
	if p.Surname != "ERIKSSON" || p.GivenNames != "ANNA MARIA" || p.PassportNumber != "L898902C3" || p.BirthDate != "740812" || p.ExpiryDate != "120415" {
		t.Fatalf("incorrect fields: %+v", p)
	}
	if !p.AllChecksPassed {
		t.Fatalf("failed checks: %+v", p.Checks)
	}
}

func TestFieldCorruption(t *testing.T) {
	for name, position := range map[string]int{"passport_number": 0, "birth_date": 13, "expiry_date": 21, "optional_data": 28} {
		t.Run(name, func(t *testing.T) {
			lines := sampleMRZ()
			b := []byte(lines[1])
			if b[position] == 'Z' || b[position] == '9' {
				b[position]--
			} else {
				b[position]++
			}
			lines[1] = string(b)
			p, err := ParseTD3(lines)
			if err != nil {
				t.Fatal(err)
			}
			if p.AllChecksPassed || p.Checks[name].Passed || p.Checks["composite"].Passed {
				t.Fatalf("corruption was accepted: %+v", p.Checks)
			}
		})
	}
}

func TestStructuralErrors(t *testing.T) {
	for _, lines := range [][]string{
		nil, {sampleMRZ()[0]}, {sampleMRZ()[0], sampleMRZ()[1][:43]},
		{strings.Repeat("<", 44), sampleMRZ()[1]},
		{sampleMRZ()[0], "!" + sampleMRZ()[1][1:]},
		{sampleMRZ()[0], sampleMRZ()[1][:13] + "O" + sampleMRZ()[1][14:]},
		{sampleMRZ()[0], sampleMRZ()[1][:20] + "X" + sampleMRZ()[1][21:]},
	} {
		if _, err := ParseTD3(lines); err == nil {
			t.Fatalf("accepted malformed input: %q", lines)
		}
	}
}

func TestUnusedOptionalData(t *testing.T) {
	for _, digit := range []byte{'0', '<'} {
		lines := sampleMRZ()
		line := lines[1][:28] + strings.Repeat("<", 14) + string(digit)
		line += string(calculateCheckDigit(line[0:10] + line[13:20] + line[21:43]))
		lines[1] = line
		p, err := ParseTD3(lines)
		if err != nil {
			t.Fatal(err)
		}
		if !p.AllChecksPassed {
			t.Fatalf("unused optional data failed: %+v", p.Checks)
		}
	}
}

func TestExtractionWithoutFillers(t *testing.T) {
	line2 := strings.ReplaceAll(sampleMRZ()[1], "<", "0")
	lines := extractMRZ("heading\r\n" + sampleMRZ()[0] + "\r\n" + line2 + "\n")
	if len(lines) != 2 || lines[1] != line2 {
		t.Fatalf("lost second line: %q", lines)
	}
}

func TestResponseFields(t *testing.T) {
	for _, lines := range [][]string{sampleMRZ(), nil} {
		out := map[string]any{}
		addMRZResult(out, lines)
		if _, err := json.Marshal(out); err != nil {
			t.Fatal(err)
		}
		if len(lines) == 0 && out["parse_error"] == nil {
			t.Fatal("missing parse error")
		}
		if len(lines) == 2 && out["parse_error"] != nil {
			t.Fatal(out["parse_error"])
		}
	}
}

func TestPassportOutputFields(t *testing.T) {
	out := map[string]any{}
	addMRZResult(out, sampleMRZ())
	expected := map[string]string{
		"first_name": "ANNA MARIA", "last_name": "ERIKSSON",
		"passport_number": "L898902C3", "date_of_birth": "740812",
		"gender": "F", "nationality": "UTO", "passport_expiration_date": "120415",
	}
	for field, want := range expected {
		if out[field] != want {
			t.Errorf("%s: got %v, want %s", field, out[field], want)
		}
	}
	validation := out["validation"].(map[string]any)
	if validation["status"] != "checks_passed" || validation["all_checks_passed"] != true {
		t.Fatalf("unexpected validation: %v", validation)
	}
	lines := sampleMRZ()
	lines[1] = "M" + lines[1][1:]
	addMRZResult(out, lines)
	validation = out["validation"].(map[string]any)
	if out["passport_number"] != "M898902C3" || validation["status"] != "check_digits_failed" {
		t.Fatalf("missing fields or checksum failure: %v", out)
	}
	addMRZResult(out, nil)
	for field := range expected {
		value, exists := out[field]
		if !exists || value != nil {
			t.Errorf("%s should be present and null on parse failure", field)
		}
	}
	if out["validation"].(map[string]any)["status"] != "parse_failed" {
		t.Fatal("missing parse_failed status")
	}
}
