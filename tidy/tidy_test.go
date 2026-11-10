package tidy

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseNumber(t *testing.T) {
	ok := map[string]float64{
		"1234":      1234,
		"1 234,50":  1234.5,
		"1 234,50":  1234.5,
		"1,234.50":  1234.5,
		"1.234,50":  1234.5,
		"2 500 ₽":   2500,
		"2500 руб.": 2500,
		"$15":       15,
		"-12":       -12,
		"−12,5":     -12.5,
		"(12)":      -12,
		"+7":        7,
		"1,5":       1.5,
		"1.5":       1.5,
		"1,234,567": 1234567,
		"1.234.567": 1234567,
		"15%":       15,
		"0,001":     0.001,
	}
	for in, want := range ok {
		got, valid := ParseNumber(in)
		if !valid || got != want {
			t.Errorf("ParseNumber(%q) = %v, %v; want %v", in, got, valid, want)
		}
	}
	for _, in := range []string{"", " ", "abc", "12abc", "договорная", "1,2,3.4", "1.2.3", "12,34,567", "1.234,5,6", ",5", "5,", "--5", "-", "1e5", "12-3", "1.2,3.4"} {
		if v, valid := ParseNumber(in); valid {
			t.Errorf("ParseNumber(%q) = %v, want failure", in, v)
		}
	}
}

func TestParseDate(t *testing.T) {
	ok := map[string]string{
		"05.03.2024":       "2024-03-05",
		"5.3.24":           "2024-03-05",
		"05/03/2024":       "2024-03-05",
		"2024-03-05":       "2024-03-05",
		"05-03-2024":       "2024-03-05",
		"2024.03.05":       "2024-03-05",
		"05.03.2024 14:30": "2024-03-05",
		"2024-03-05T10:00": "2024-03-05",
		" 29.02.2024 ":     "2024-02-29",
	}
	for in, want := range ok {
		got, valid := ParseDate(in)
		if !valid || FormatDate(got) != want {
			t.Errorf("ParseDate(%q) = %v, %v; want %s", in, got, valid, want)
		}
	}
	for _, in := range []string{"", "31.02.2024", "29.02.2023", "32.01.2024", "12345", "1.5", "01.01.1800", "01.01.2200", "март 2024", "13.13.2024"} {
		if d, valid := ParseDate(in); valid {
			t.Errorf("ParseDate(%q) = %v, want failure", in, d)
		}
	}
}

func TestDayFirst(t *testing.T) {
	d, _ := ParseDate("03.04.2024")
	if FormatDate(d) != "2024-04-03" {
		t.Errorf("03.04.2024 must be 3 April, got %s", FormatDate(d))
	}
}

func TestDecode(t *testing.T) {
	// "Привет, Ёж №1" in Windows-1251
	raw := []byte{0xCF, 0xF0, 0xE8, 0xE2, 0xE5, 0xF2, ',', ' ', 0xA8, 0xE6, ' ', 0xB9, '1'}
	got, enc := Decode(raw)
	if enc != CP1251 || got != "Привет, Ёж №1" {
		t.Errorf("cp1251: got %q, %s", got, enc)
	}
	got, enc = Decode([]byte("\xEF\xBB\xBFЯ;b"))
	if enc != UTF8 || got != "Я;b" {
		t.Errorf("utf-8 with BOM: got %q, %s", got, enc)
	}
	got, enc = Decode([]byte("plain ascii"))
	if enc != UTF8 || got != "plain ascii" {
		t.Errorf("ascii: got %q, %s", got, enc)
	}
}

func TestCP1251TableComplete(t *testing.T) {
	if len(cp1251High) != 128 {
		t.Fatalf("table has %d entries, want 128", len(cp1251High))
	}
	all := make([]byte, 0, 64)
	for b := 0xC0; b <= 0xFF; b++ {
		all = append(all, byte(b))
	}
	got, _ := Decode(all)
	if got != "АБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯабвгдежзийклмнопрстуфхцчшщъыьэюя" {
		t.Errorf("letters mismatch: %q", got)
	}
}

func TestDetectDelimiter(t *testing.T) {
	cases := map[string]rune{
		"a;b;c\n1;2;3":     ';',
		"a,b,c\n1,2,3":     ',',
		"a\tb\tc":          '\t',
		"a|b|c":            '|',
		"one column\nx":    ',',
		`"a;b",c,d` + "\n": ',',
		"a;b,c;d\n1;2,3;4": ';',
		"":                 ',',
	}
	for in, want := range cases {
		if got := DetectDelimiter(in); got != want {
			t.Errorf("DetectDelimiter(%q) = %q, want %q", in, got, want)
		}
	}
}

func parse(t *testing.T, s string) *Table {
	t.Helper()
	tb, err := Parse([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return tb
}

func TestParseTable(t *testing.T) {
	tb := parse(t, "A;;A\n1;2\n\n  ;  ;  \n3;4;5;6\n")
	if got := strings.Join(tb.Header, "|"); got != "A|колонка 2|A (2)|колонка 4" {
		t.Errorf("header = %q", got)
	}
	// blank lines are dropped, rows padded to the widest one
	if len(tb.Rows) != 2 {
		t.Fatalf("rows = %v", tb.Rows)
	}
	for _, r := range tb.Rows {
		if len(r) != 4 {
			t.Errorf("row not rectangular: %v", r)
		}
	}
	if _, err := Parse([]byte("  \n\n")); err == nil {
		t.Error("empty file must be an error")
	}
}

func TestQuotedFields(t *testing.T) {
	tb := parse(t, "a;b\n\"x;y\";\"he said \"\"hi\"\"\"\n")
	if tb.Rows[0][0] != "x;y" || tb.Rows[0][1] != `he said "hi"` {
		t.Errorf("quoted fields: %v", tb.Rows[0])
	}
}

func TestClean(t *testing.T) {
	src := "id;date;sum;who\n" +
		"1;05.03.2024;1 234,50;  Анна   К. \n" +
		"2;2024-03-06;2 500 руб.;Борис\n" +
		"2;06/03/2024;2500;Борис\n" + // duplicate once normalized
		"3;31.02.2024;договорная;Борис\n" +
		"4;07.03.2024;;\n" +
		"5;08.03.2024;100;Вера\n"
	tb := parse(t, src)
	rep := tb.Clean()
	if rep.RowsIn != 6 || rep.RowsOut != 5 || rep.Duplicates != 1 {
		t.Errorf("report = %+v", rep)
	}
	want := [][]string{
		{"1", "2024-03-05", "1234.5", "Анна К."},
		{"2", "2024-03-06", "2500", "Борис"},
		{"3", "31.02.2024", "договорная", "Борис"},
		{"4", "2024-03-07", "", ""},
		{"5", "2024-03-08", "100", "Вера"},
	}
	for i, w := range want {
		if strings.Join(tb.Rows[i], "|") != strings.Join(w, "|") {
			t.Errorf("row %d = %v, want %v", i, tb.Rows[i], w)
		}
	}
	if len(rep.Problems) != 2 {
		t.Fatalf("problems = %+v", rep.Problems)
	}
	if p := rep.Problems[0]; p.Row != 4 || p.Column != "date" || p.Reason != "не дата" {
		t.Errorf("problem 0 = %+v", p)
	}
	if p := rep.Problems[1]; p.Row != 4 || p.Column != "sum" || p.Reason != "не число" {
		t.Errorf("problem 1 = %+v", p)
	}
	kinds := []Kind{KindNumber, KindDate, KindNumber, KindText}
	for i, k := range kinds {
		if rep.Kinds[i] != k {
			t.Errorf("kind %d = %s, want %s", i, rep.Kinds[i], k)
		}
	}
}

func TestCleanIsIdempotent(t *testing.T) {
	tb := parse(t, "d;s\n05.03.2024;1 234,5\n6.3.24;2,5\n6.3.24;2,5\n")
	tb.Clean()
	var first bytes.Buffer
	if err := tb.Write(&first, ','); err != nil {
		t.Fatal(err)
	}
	again := parse(t, first.String())
	rep := again.Clean()
	if rep.Changed != 0 || rep.Duplicates != 0 || len(rep.Problems) != 0 {
		t.Errorf("second pass changed things: %+v", rep)
	}
}

func TestInferKind(t *testing.T) {
	cases := []struct {
		vals []string
		want Kind
	}{
		{[]string{"1", "2,5", "x", "4", "5"}, KindNumber}, // 4 of 5 = 80%
		{[]string{"1", "x", "y", "4"}, KindText},
		{[]string{"05.03.2024", "6.3.24", ""}, KindDate},
		{[]string{"", " ", ""}, KindEmpty},
		{[]string{"12345", "67890"}, KindNumber}, // long numbers are not dates
		{nil, KindEmpty},
	}
	for _, c := range cases {
		if got := InferKind(c.vals); got != c.want {
			t.Errorf("InferKind(%q) = %s, want %s", c.vals, got, c.want)
		}
	}
}

func TestStats(t *testing.T) {
	tb := parse(t, "n;d;t\n5;01.02.2024;a\n-2;03.01.2024;A\n;;b\n9;;a\n")
	st := tb.Stats()
	if st[0].Kind != KindNumber || st[0].Filled != 3 || st[0].Empty != 1 || st[0].Min != "-2" || st[0].Max != "9" {
		t.Errorf("numeric stats: %+v", st[0])
	}
	if st[1].Kind != KindDate || st[1].Min != "2024-01-03" || st[1].Max != "2024-02-01" {
		t.Errorf("date stats: %+v", st[1])
	}
	if st[2].Kind != KindText || st[2].Unique != 2 || st[2].Top != "A (3)" {
		t.Errorf("text stats: %+v", st[2])
	}
}

func TestGroupSum(t *testing.T) {
	tb := parse(t, "who;amt\nАнна;10\nборис;5,5\nАнна ;2\nБорис;x\n;1\nВера;12\n")
	groups, skipped, err := tb.GroupSum("WHO", "amt")
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1", skipped)
	}
	got := make([]string, len(groups))
	for i, g := range groups {
		got[i] = g.Key + ":" + FormatNumber(g.Total)
	}
	// Анна and "Анна " are one group; sorted by total, ties by key
	want := "Анна:12|Вера:12|борис:5.5|(пусто):1"
	if strings.Join(got, "|") != want {
		t.Errorf("groups = %v, want %s", got, want)
	}
	if _, _, err := tb.GroupSum("nope", "amt"); err == nil {
		t.Error("unknown column must be an error")
	}
	if _, _, err := tb.GroupSum("who", "nope"); err == nil {
		t.Error("unknown sum column must be an error")
	}
}

func TestWriteDelimiter(t *testing.T) {
	tb := parse(t, "a;b\n\"x;y\";2\n")
	var buf bytes.Buffer
	if err := tb.Write(&buf, ';'); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "a;b\n\"x;y\";2\n" {
		t.Errorf("write = %q", got)
	}
}
