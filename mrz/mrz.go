// Package mrz parses and validates ICAO 9303 machine readable zones
// (TD3 = passport 2x44, TD2 = 2x36, TD1 = ID card 3x30).
//
// Pipeline: Normalize -> detect format -> fix characters by field type -> parse -> check digits.
package mrz

import (
	"fmt"
	"strings"
	"time"
)

// ---------- result types ----------

// Check is one check-digit verification.
type Check struct {
	Name     string `json:"name"`
	Data     string `json:"data"`
	Expected string `json:"expected"` // digit we computed
	Got      string `json:"got"`      // digit printed in the MRZ
	OK       bool   `json:"ok"`
}

type Document struct {
	Format         string   `json:"format"` // TD1 / TD2 / TD3
	Lines          []string `json:"lines"`  // lines after normalization and correction
	DocType        string   `json:"doc_type"`
	IssuingCountry string   `json:"issuing_country"`
	Surname        string   `json:"surname"`
	GivenNames     string   `json:"given_names"`
	DocNumber      string   `json:"doc_number"`
	Nationality    string   `json:"nationality"`
	Sex            string   `json:"sex"`
	BirthDate      string   `json:"birth_date_raw"`  // YYMMDD as printed
	ExpiryDate     string   `json:"expiry_date_raw"` // YYMMDD as printed
	Birth          string   `json:"birth_date"`      // YYYY-MM-DD, empty if invalid
	Expiry         string   `json:"expiry_date"`
	PersonalNumber string   `json:"personal_number"`
	Checks         []Check  `json:"checks"`
	Issues         []string `json:"issues"`
}

// Score returns how many check digits passed out of the total.
func (d *Document) Score() (passed, total int) {
	for _, c := range d.Checks {
		total++
		if c.OK {
			passed++
		}
	}
	return
}

// Valid is true only if every check digit passed and no other issue was found.
func (d *Document) Valid() bool {
	p, t := d.Score()
	return t > 0 && p == t && len(d.Issues) == 0
}

// ---------- check digit ----------

// CheckDigit computes the ICAO 9303 check digit (weights 7,3,1; digits=value,
// A-Z=10..35, '<'=0). ok is false if s contains any other character.
func CheckDigit(s string) (digit int, ok bool) {
	weights := [3]int{7, 3, 1}
	sum := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		var v int
		switch {
		case c >= '0' && c <= '9':
			v = int(c - '0')
		case c >= 'A' && c <= 'Z':
			v = int(c-'A') + 10
		case c == '<':
			v = 0
		default:
			return 0, false
		}
		sum += v * weights[i%3]
	}
	return sum % 10, true
}

func verify(name, data string, got byte, fillerOK bool) Check {
	c := Check{Name: name, Data: data, Got: string(got)}
	// An empty optional field may carry '<' or '0' as its check digit.
	if fillerOK && strings.Trim(data, "<") == "" && (got == '<' || got == '0') {
		c.Expected, c.OK = string(got), true
		return c
	}
	d, ok := CheckDigit(data)
	if !ok {
		c.Expected = "?"
		return c
	}
	c.Expected = fmt.Sprint(d)
	c.OK = c.Expected == c.Got
	return c
}

// ---------- normalization ----------

// Normalize uppercases, drops whitespace, and turns characters that are not
// A-Z, 0-9 or '<' into '<' (OCR often reads the filler as ( { [ | « etc.).
// It also fixes K/C sitting alone between fillers, a very common OCR error.
func Normalize(s string) string {
	var b []byte
	for _, r := range strings.ToUpper(s) {
		switch {
		case r == '$': // Tesseract commonly reads the passport letter S as a dollar sign.
			b = append(b, 'S')
		case r == ' ' || r == '\t' || r == '\r' || r == '\n':
			continue
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '<':
			b = append(b, byte(r))
		default:
			b = append(b, '<')
		}
	}
	for pass := 0; pass < 2; pass++ {
		for i := range b {
			if (b[i] == 'K' || b[i] == 'C') && i > 0 && b[i-1] == '<' &&
				(i == len(b)-1 || b[i+1] == '<') {
				b[i] = '<'
			}
		}
	}
	return string(b)
}

// ---------- layouts ----------

type kind int

const (
	alpha kind = iota
	numeric
	alnum
)

type span struct {
	line, from, to int
	k              kind
}

type layout struct {
	name  string
	lines int
	width int
	spans []span
}

var td3 = layout{"TD3", 2, 44, []span{
	{0, 0, 2, alpha}, {0, 2, 5, alpha}, {0, 5, 44, alpha},
	{1, 0, 9, alnum}, {1, 9, 10, numeric}, {1, 10, 13, alpha}, {1, 13, 19, numeric},
	{1, 19, 20, numeric}, {1, 20, 21, alpha}, {1, 21, 27, numeric}, {1, 27, 28, numeric},
	{1, 28, 42, alnum}, {1, 42, 43, numeric}, {1, 43, 44, numeric},
}}

var td2 = layout{"TD2", 2, 36, []span{
	{0, 0, 2, alpha}, {0, 2, 5, alpha}, {0, 5, 36, alpha},
	{1, 0, 9, alnum}, {1, 9, 10, numeric}, {1, 10, 13, alpha}, {1, 13, 19, numeric},
	{1, 19, 20, numeric}, {1, 20, 21, alpha}, {1, 21, 27, numeric}, {1, 27, 28, numeric},
	{1, 28, 35, alnum}, {1, 35, 36, numeric},
}}

var td1 = layout{"TD1", 3, 30, []span{
	{0, 0, 2, alpha}, {0, 2, 5, alpha}, {0, 5, 14, alnum}, {0, 14, 15, numeric}, {0, 15, 30, alnum},
	{1, 0, 6, numeric}, {1, 6, 7, numeric}, {1, 7, 8, alpha}, {1, 8, 14, numeric},
	{1, 14, 15, numeric}, {1, 15, 18, alpha}, {1, 18, 29, alnum}, {1, 29, 30, numeric},
	{2, 0, 30, alpha},
}}

// Letters that OCR confuses with digits, and the other way around.
var toDigit = map[byte]byte{'O': '0', 'Q': '0', 'D': '0', 'I': '1', 'L': '1', 'Z': '2', 'S': '5', 'G': '6', 'B': '8'}
var toLetter = map[byte]byte{'0': 'O', '1': 'I', '2': 'Z', '5': 'S', '6': 'G', '8': 'B'}

// detect finds the format and picks the lines of the right width.
func detect(lines []string) (*layout, []string, error) {
	for _, l := range []*layout{&td3, &td1, &td2} {
		var m []string
		for _, s := range lines {
			if len(s) == l.width-1 {
				s += "<"
			} else if len(s) == l.width+1 && strings.HasSuffix(s, "<") {
				s = s[:l.width]
			} else if len(s) > l.width && len(s) <= l.width+4 {
				s = s[:l.width]
			}
			if len(s) == l.width {
				m = append(m, s)
			}
		}
		if len(m) >= l.lines {
			return l, m[len(m)-l.lines:], nil
		}
	}
	var lens []int
	for _, s := range lines {
		lens = append(lens, len(s))
	}
	return nil, nil, fmt.Errorf("no MRZ block found: line lengths %v (expected 2x44, 2x36 or 3x30)", lens)
}

// correct replaces letters with digits inside numeric fields and digits with
// letters inside alphabetic fields, using the known field layout.
func correct(l *layout, lines []string) []string {
	out := make([][]byte, len(lines))
	for i, s := range lines {
		out[i] = []byte(s)
	}
	for _, sp := range l.spans {
		for i := sp.from; i < sp.to; i++ {
			c := out[sp.line][i]
			switch sp.k {
			case numeric:
				if r, ok := toDigit[c]; ok {
					out[sp.line][i] = r
				}
			case alpha:
				if r, ok := toLetter[c]; ok {
					out[sp.line][i] = r
				}
			}
		}
	}
	res := make([]string, len(out))
	for i, b := range out {
		res[i] = string(b)
	}
	return res
}

// ---------- parsing ----------

func trimFill(s string) string { return strings.TrimRight(s, "<") }

func splitName(s string) (surname, given string) {
	s = trimFill(s)
	parts := strings.SplitN(s, "<<", 2)
	surname = strings.Join(strings.Fields(strings.ReplaceAll(parts[0], "<", " ")), " ")
	if len(parts) == 2 {
		// Within given names, single fillers separate names; a double filler
		// starts trailing padding. Ignore OCR garbage beyond that boundary.
		givenPart := strings.SplitN(parts[1], "<<", 2)[0]
		given = strings.Join(strings.Fields(strings.ReplaceAll(givenPart, "<", " ")), " ")
	}
	return
}

func parseDate(s string, birth bool, now time.Time) (string, error) {
	if len(s) != 6 {
		return "", fmt.Errorf("date %q is not 6 characters", s)
	}
	for i := 0; i < 6; i++ {
		if s[i] < '0' || s[i] > '9' {
			return "", fmt.Errorf("date %q has non-digits", s)
		}
	}
	yy := int(s[0]-'0')*10 + int(s[1]-'0')
	mm := int(s[2]-'0')*10 + int(s[3]-'0')
	dd := int(s[4]-'0')*10 + int(s[5]-'0')
	year := 2000 + yy
	if birth && year > now.Year() { // two-digit year in the future => 1900s
		year -= 100
	}
	t := time.Date(year, time.Month(mm), dd, 0, 0, 0, 0, time.UTC)
	if int(t.Month()) != mm || t.Day() != dd {
		return "", fmt.Errorf("date %q is not a real date", s)
	}
	return t.Format("2006-01-02"), nil
}

// Parse takes the candidate lines an OCR engine produced (they may contain
// noise lines, spaces, odd characters) and returns the parsed document.
// The error is non-nil if no MRZ block is found or fields are structurally
// invalid. If only check digits fail, Parse still returns the document and
// you decide with doc.Valid() / doc.Score().
func Parse(raw []string) (*Document, error) {
	var norm []string
	for _, r := range raw {
		if n := Normalize(r); n != "" {
			norm = append(norm, n)
		}
	}
	l, block, err := detect(norm)
	if err != nil {
		return nil, err
	}
	ls := correct(l, block)
	d := &Document{Format: l.name, Lines: ls}
	now := time.Now()

	var birth, expiry string
	switch l.name {
	case "TD3":
		l1, l2 := ls[0], ls[1]
		d.DocType, d.IssuingCountry = trimFill(l1[0:2]), trimFill(l1[2:5])
		d.Surname, d.GivenNames = splitName(l1[5:44])
		d.DocNumber, d.Nationality = trimFill(l2[0:9]), trimFill(l2[10:13])
		birth, d.Sex, expiry = l2[13:19], trimFill(l2[20:21]), l2[21:27]
		d.PersonalNumber = trimFill(l2[28:42])
		d.Checks = []Check{
			verify("document number", l2[0:9], l2[9], false),
			verify("birth date", l2[13:19], l2[19], false),
			verify("expiry date", l2[21:27], l2[27], false),
			verify("personal number", l2[28:42], l2[42], true),
			verify("composite", l2[0:10]+l2[13:20]+l2[21:43], l2[43], false),
		}
	case "TD2":
		l1, l2 := ls[0], ls[1]
		d.DocType, d.IssuingCountry = trimFill(l1[0:2]), trimFill(l1[2:5])
		d.Surname, d.GivenNames = splitName(l1[5:36])
		d.DocNumber, d.Nationality = trimFill(l2[0:9]), trimFill(l2[10:13])
		birth, d.Sex, expiry = l2[13:19], trimFill(l2[20:21]), l2[21:27]
		d.PersonalNumber = trimFill(l2[28:35])
		d.Checks = []Check{
			verify("document number", l2[0:9], l2[9], false),
			verify("birth date", l2[13:19], l2[19], false),
			verify("expiry date", l2[21:27], l2[27], false),
			verify("composite", l2[0:10]+l2[13:20]+l2[21:35], l2[35], false),
		}
	case "TD1":
		l1, l2, l3 := ls[0], ls[1], ls[2]
		d.DocType, d.IssuingCountry = trimFill(l1[0:2]), trimFill(l1[2:5])
		d.DocNumber = trimFill(l1[5:14])
		d.PersonalNumber = trimFill(l1[15:30])
		birth, d.Sex, expiry = l2[0:6], trimFill(l2[7:8]), l2[8:14]
		d.Nationality = trimFill(l2[15:18])
		d.Surname, d.GivenNames = splitName(l3)
		d.Checks = []Check{
			verify("document number", l1[5:14], l1[14], false),
			verify("birth date", l2[0:6], l2[6], false),
			verify("expiry date", l2[8:14], l2[14], false),
			verify("composite", l1[5:30]+l2[0:7]+l2[8:15]+l2[18:29], l2[29], false),
		}
	}

	d.BirthDate, d.ExpiryDate = birth, expiry
	if v, err := parseDate(birth, true, now); err != nil {
		d.Issues = append(d.Issues, "birth date: "+err.Error())
	} else {
		d.Birth = v
	}
	if v, err := parseDate(expiry, false, now); err != nil {
		d.Issues = append(d.Issues, "expiry date: "+err.Error())
	} else {
		d.Expiry = v
	}
	if d.Sex != "M" && d.Sex != "F" && d.Sex != "X" && d.Sex != "" {
		d.Issues = append(d.Issues, fmt.Sprintf("sex %q is not M/F/X", d.Sex))
	}
	if len(d.Nationality) != 3 || strings.IndexFunc(d.Nationality, func(r rune) bool { return r < 'A' || r > 'Z' }) >= 0 {
		d.Issues = append(d.Issues, "nationality is not a three-letter code")
	}
	if d.Surname == "" {
		d.Issues = append(d.Issues, "empty surname")
	}
	if len(d.Issues) > 0 {
		return nil, fmt.Errorf("invalid MRZ structure (OCR may have dropped or shifted characters): %s", strings.Join(d.Issues, "; "))
	}
	return d, nil
}
