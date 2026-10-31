package mrz

import "strings"

// Checksum compares the digit printed in the MRZ with the calculated digit.
// A missing or nonnumeric printed digit is represented as null.
type Checksum struct {
	Expected   *int `json:"expected"`
	Calculated int  `json:"calculated"`
	IsValid    bool `json:"isValid"`
}

type Passport struct {
	FirstName              string    `json:"firstName"`
	LastName               string    `json:"lastName"`
	FullNameLatin          string    `json:"fullNameLatin"`
	PassportNumber         string    `json:"passportNumber"`
	BirthDate              string    `json:"birthDate"`
	ExpiryDate             string    `json:"expiryDate"`
	Gender                 string    `json:"gender"`
	Nationality            string    `json:"nationality"`
	MRZRawText             string    `json:"mrzRawText"`
	PassportNumberChecksum *Checksum `json:"passportNumberChecksum"`
	BirthDateChecksum      *Checksum `json:"birthDateChecksum"`
	ExpiryDateChecksum     *Checksum `json:"expiryDateChecksum"`
	PersonalNumberChecksum *Checksum `json:"personalNumberChecksum"`
	CompositeChecksum      *Checksum `json:"compositeChecksum"`
	AllChecksumsValid      bool      `json:"allChecksumsValid"`
}

func (d *Document) Passport() Passport {
	passed, total := d.Score()
	p := Passport{
		FirstName: d.GivenNames, LastName: d.Surname,
		FullNameLatin:  strings.TrimSpace(d.GivenNames + " " + d.Surname),
		PassportNumber: d.DocNumber, BirthDate: d.Birth, ExpiryDate: d.Expiry,
		Gender: d.Sex, Nationality: d.Nationality,
		MRZRawText:        strings.Join(d.Lines, "\n"),
		AllChecksumsValid: total > 0 && passed == total,
	}
	for _, c := range d.Checks {
		calculated, _ := CheckDigit(c.Data)
		check := &Checksum{Calculated: calculated, IsValid: c.OK}
		if len(c.Got) == 1 && c.Got[0] >= '0' && c.Got[0] <= '9' {
			digit := int(c.Got[0] - '0')
			check.Expected = &digit
		} else if c.Name == "personal number" && c.Got == "<" && strings.Trim(c.Data, "<") == "" {
			// ICAO permits a filler instead of zero for an unused optional field.
			zero := 0
			check.Expected = &zero
		}
		switch c.Name {
		case "document number":
			p.PassportNumberChecksum = check
		case "birth date":
			p.BirthDateChecksum = check
		case "expiry date":
			p.ExpiryDateChecksum = check
		case "personal number":
			p.PersonalNumberChecksum = check
		case "composite":
			p.CompositeChecksum = check
		}
	}
	return p
}

const (
	StatusCompleted = 2
	StatusFailed    = 3
)

// Result is the response envelope for both successful and failed requests.
type Result struct {
	RequestID      int64     `json:"requestId"`
	JobID          int64     `json:"jobId"`
	Status         int       `json:"status"`
	PassportResult *Passport `json:"passportResult"`
	GeneralResult  any       `json:"generalResult"`
	Error          *string   `json:"error"`
}

func Failure(requestID, jobID int64, message string) Result {
	return Result{RequestID: requestID, JobID: jobID, Status: StatusFailed, Error: &message}
}

// Response parses OCR text. Checksum failures are returned in the passport
// result so callers can distinguish recognition from checksum validation.
func Response(requestID, jobID int64, lines []string) Result {
	d, err := Parse(lines)
	if err != nil {
		return Failure(requestID, jobID, err.Error())
	}
	passport := d.Passport()
	return Result{RequestID: requestID, JobID: jobID, Status: StatusCompleted, PassportResult: &passport}
}
