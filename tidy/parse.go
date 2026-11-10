package tidy

import (
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// NormalizeSpace trims a cell and collapses runs of whitespace
// (including non-breaking spaces) into a single space.
func NormalizeSpace(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == '\u00A0' || r == '\u200B' || r == '\uFEFF'
	}), " ")
}

var currencyWords = []string{"руб.", "руб", "р.", "₽", "$", "€", "usd", "eur", "rub"}

// ParseNumber understands numbers as people write them in spreadsheets:
// "1 234,50", "1,234.50", "1.234,50", "2 500 ₽", "-12", "(12)", "15%".
//
// Rules for separators:
//   - both "." and "," present: the one that comes last is the decimal mark;
//   - one kind of separator, once: it is the decimal mark ("1,5", "1.5");
//   - one kind of separator, several times: it groups thousands ("1,234,567"),
//     and the groups must be of three digits, so "1.2.3" is not a number.
func ParseNumber(s string) (float64, bool) {
	s = strings.ToLower(NormalizeSpace(s))
	if s == "" {
		return 0, false
	}
	neg := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		neg = true
		s = s[1 : len(s)-1]
	}
	s = strings.TrimSuffix(s, "%")
	for _, w := range currencyWords {
		s = strings.ReplaceAll(s, w, "")
	}
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "−", "-") // unicode minus
	if strings.HasPrefix(s, "-") {
		neg = !neg
		s = s[1:]
	} else if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9') && r != '.' && r != ',' {
			return 0, false
		}
	}
	if s[0] == '.' || s[0] == ',' || s[len(s)-1] == '.' || s[len(s)-1] == ',' {
		return 0, false
	}
	dots, commas := strings.Count(s, "."), strings.Count(s, ",")
	// intPart returns the digits before the decimal mark (or all digits) with group marks.
	groupsOK := func(intPart, sep string) bool {
		parts := strings.Split(intPart, sep)
		if len(parts[0]) < 1 || len(parts[0]) > 3 {
			return false
		}
		for _, p := range parts[1:] {
			if len(p) != 3 {
				return false
			}
		}
		return true
	}
	lastDot, lastComma := strings.LastIndex(s, "."), strings.LastIndex(s, ",")
	switch {
	case dots > 0 && commas > 0:
		dec := ','
		group := "."
		if lastDot > lastComma {
			dec, group = '.', ","
		}
		// the decimal mark must appear once, after every group mark
		if strings.Count(s, string(dec)) != 1 {
			return 0, false
		}
		if strings.LastIndex(s, group) > strings.IndexRune(s, dec) {
			return 0, false
		}
		if !groupsOK(s[:strings.IndexRune(s, dec)], group) {
			return 0, false
		}
		s = strings.ReplaceAll(s, group, "")
		s = strings.Replace(s, string(dec), ".", 1)
	case dots > 1:
		if !groupsOK(s, ".") {
			return 0, false
		}
		s = strings.ReplaceAll(s, ".", "")
	case commas > 1:
		if !groupsOK(s, ",") {
			return 0, false
		}
		s = strings.ReplaceAll(s, ",", "")
	case commas == 1:
		s = strings.Replace(s, ",", ".", 1)
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
		return 0, false
	}
	if neg {
		v = -v
	}
	return v, true
}

// FormatNumber prints a number without trailing zeros: 1234.5, 20, -3.25.
func FormatNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

var dateLayouts = []string{
	"2.1.2006", "2.1.06", "2006-1-2", "2/1/2006", "2-1-2006", "2006.1.2", "2006/1/2",
}

// ParseDate reads a day-first date ("05.03.2024", "5.3.24", "05/03/2024")
// or an ISO date ("2024-03-05"). A time of day after the date is ignored.
// Years outside 1900..2100 are rejected so that plain numbers are not mistaken for dates.
func ParseDate(s string) (time.Time, bool) {
	s = NormalizeSpace(s)
	if i := strings.IndexAny(s, " T"); i > 0 {
		s = s[:i]
	}
	if len(s) < 6 {
		return time.Time{}, false
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			if y := t.Year(); y >= 1900 && y <= 2100 {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

// FormatDate prints a date as ISO 8601 (2024-03-05).
func FormatDate(t time.Time) string { return t.Format("2006-01-02") }
