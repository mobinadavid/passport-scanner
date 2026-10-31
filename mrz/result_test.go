package mrz

import (
	"strings"
	"testing"
)

func TestPassportChecksums(t *testing.T) {
	lines := []string{"P<UTOERIKSSON<<ANNA<MARIA<<<<<<<<<<<<<<<<<<<<<", "L898902C36UTO7408122F1204159ZE184226B<<<<<10"}
	p := Response(1, 2, lines).PassportResult
	if p == nil || !p.AllChecksumsValid {
		t.Fatalf("unexpected passport: %+v", p)
	}
	for _, c := range []*Checksum{p.PassportNumberChecksum, p.BirthDateChecksum, p.ExpiryDateChecksum, p.PersonalNumberChecksum, p.CompositeChecksum} {
		if c == nil || c.Expected == nil || !c.IsValid || *c.Expected != c.Calculated {
			t.Fatalf("unexpected checksum: %+v", c)
		}
	}
	lines[1] = "M" + lines[1][1:]
	p = Response(1, 2, lines).PassportResult
	if p.AllChecksumsValid || p.PassportNumberChecksum.IsValid || p.CompositeChecksum.IsValid {
		t.Fatalf("corruption accepted: %+v", p)
	}
	if *p.PassportNumberChecksum.Expected != 6 || p.PassportNumberChecksum.Calculated == 6 {
		t.Fatalf("printed and calculated digits reversed: %+v", p.PassportNumberChecksum)
	}
}

func TestUnusedPersonalNumber(t *testing.T) {
	for _, digit := range []string{"0", "<"} {
		line := "L898902C36UTO7408122F1204159" + strings.Repeat("<", 14) + digit
		check, _ := CheckDigit(line[:10] + line[13:20] + line[21:43])
		line += string(byte('0' + check))
		p := Response(1, 2, []string{"P<UTOERIKSSON<<ANNA<MARIA<<<<<<<<<<<<<<<<<<<<<", line}).PassportResult
		if p == nil || !p.AllChecksumsValid || *p.PersonalNumberChecksum.Expected != 0 || p.PersonalNumberChecksum.Calculated != 0 {
			t.Fatalf("unused field: %+v", p)
		}
	}
}
