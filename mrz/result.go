package mrz

import "fmt"

// Passport is the data we return to the client.
type Passport struct {
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	PassportNumber string `json:"passport_number"`
	DateOfBirth    string `json:"date_of_birth"` // YYYY-MM-DD
	Gender         string `json:"gender"`        // M / F / X
	Nationality    string `json:"nationality"`   // 3-letter ICAO code, e.g. DEU
	ExpiryDate     string `json:"expiry_date"`   // YYYY-MM-DD
}

func (d *Document) Passport() Passport {
	return Passport{
		FirstName:      d.GivenNames,
		LastName:       d.Surname,
		PassportNumber: d.DocNumber,
		DateOfBirth:    d.Birth,
		Gender:         d.Sex,
		Nationality:    d.Nationality,
		ExpiryDate:     d.Expiry,
	}
}

// Response parses the OCR lines and builds the JSON body every test server returns.
// extra is merged in (raw OCR text, http status, ...).
func Response(engine string, ms int64, lines []string, extra map[string]any) map[string]any {
	out := map[string]any{"engine": engine, "ms": ms, "ocr_lines": lines, "parser_version": "2026-10-28-c"}
	for k, v := range extra {
		out[k] = v
	}
	d, err := Parse(lines)
	if err != nil {
		out["valid"] = false
		out["error"] = err.Error()
		return out
	}
	p, t := d.Score()
	out["valid"] = d.Valid()
	out["checks_passed"] = fmt.Sprintf("%d/%d", p, t)
	out["passport"] = d.Passport()
	out["mrz_lines"] = d.Lines
	out["checks"] = d.Checks
	out["issues"] = d.Issues
	return out
}
