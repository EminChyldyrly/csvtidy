package tidy

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Table is a parsed CSV file: a header and rectangular rows of text cells.
type Table struct {
	Header    []string
	Rows      [][]string
	Encoding  Encoding
	Delimiter rune
}

// DetectDelimiter picks the field separator from the first line by counting
// candidates outside of quotes. Ties and empty input fall back to ','.
func DetectDelimiter(text string) rune {
	line := text
	if i := strings.IndexAny(text, "\r\n"); i >= 0 {
		line = text[:i]
	}
	best, bestCount := ',', 0
	for _, d := range []rune{';', ',', '\t', '|'} {
		n, inQuotes := 0, false
		for _, r := range line {
			switch {
			case r == '"':
				inQuotes = !inQuotes
			case r == d && !inQuotes:
				n++
			}
		}
		if n > bestCount {
			best, bestCount = d, n
		}
	}
	return best
}

// Parse reads raw bytes (UTF-8 or Windows-1251, any common delimiter) into a Table.
// Empty lines are skipped and short rows are padded so the table is rectangular.
func Parse(data []byte) (*Table, error) {
	text, enc := Decode(data)
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("файл пустой")
	}
	delim := DetectDelimiter(text)
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	var records [][]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("ошибка разбора CSV: %w", err)
		}
		blank := true
		for _, c := range rec {
			if strings.TrimSpace(c) != "" {
				blank = false
				break
			}
		}
		if !blank {
			records = append(records, rec)
		}
	}
	if len(records) == 0 {
		return nil, errors.New("в файле нет данных")
	}
	width := 0
	for _, rec := range records {
		if len(rec) > width {
			width = len(rec)
		}
	}
	for i, rec := range records {
		for len(rec) < width {
			rec = append(rec, "")
		}
		records[i] = rec
	}
	header := records[0]
	for len(header) < width {
		header = append(header, "")
	}
	t := &Table{Header: header, Rows: records[1:], Encoding: enc, Delimiter: delim}
	t.fixHeader()
	return t, nil
}

// fixHeader trims names, names empty columns "колонка N" and makes duplicates unique.
func (t *Table) fixHeader() {
	seen := map[string]int{}
	for i, h := range t.Header {
		h = NormalizeSpace(h)
		if h == "" {
			h = fmt.Sprintf("колонка %d", i+1)
		}
		key := strings.ToLower(h)
		seen[key]++
		if seen[key] > 1 {
			h = fmt.Sprintf("%s (%d)", h, seen[key])
		}
		t.Header[i] = h
	}
}

// Col returns the index of a column by name, ignoring case and surrounding spaces.
func (t *Table) Col(name string) (int, bool) {
	want := strings.ToLower(NormalizeSpace(name))
	for i, h := range t.Header {
		if strings.ToLower(h) == want {
			return i, true
		}
	}
	return -1, false
}

// Write saves the table as UTF-8 CSV with the given delimiter.
func (t *Table) Write(w io.Writer, delim rune) error {
	cw := csv.NewWriter(w)
	cw.Comma = delim
	if err := cw.Write(t.Header); err != nil {
		return err
	}
	for _, row := range t.Rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
