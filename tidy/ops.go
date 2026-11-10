package tidy

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Kind is the inferred type of a column.
type Kind string

const (
	KindText   Kind = "текст"
	KindNumber Kind = "число"
	KindDate   Kind = "дата"
	KindEmpty  Kind = "пусто"
)

// typeShare is the share of non-empty cells that must parse for a column to count as a number or date column.
const typeShare = 0.8

// InferKind decides the type of a column from its values.
func InferKind(values []string) Kind {
	filled, nums, dates := 0, 0, 0
	for _, v := range values {
		v = NormalizeSpace(v)
		if v == "" {
			continue
		}
		filled++
		if _, ok := ParseDate(v); ok {
			dates++
		} else if _, ok := ParseNumber(v); ok {
			nums++
		}
	}
	switch {
	case filled == 0:
		return KindEmpty
	case float64(dates) >= typeShare*float64(filled):
		return KindDate
	case float64(nums) >= typeShare*float64(filled):
		return KindNumber
	}
	return KindText
}

func column(rows [][]string, i int) []string {
	out := make([]string, len(rows))
	for r, row := range rows {
		out[r] = row[i]
	}
	return out
}

// Problem is a cell that could not be converted to its column's type.
type Problem struct {
	Row    int // 1-based row number in the data, header not counted
	Column string
	Value  string
	Reason string
}

// CleanReport describes what Clean changed.
type CleanReport struct {
	RowsIn     int
	RowsOut    int
	Duplicates int
	Changed    int // cells whose text was rewritten
	Kinds      []Kind
	Problems   []Problem
}

// Clean tidies the table in place: trims and collapses spaces, writes numbers
// as 1234.5 and dates as 2024-03-05, drops rows that became exact duplicates.
// Cells that do not fit their column's type are left as they were and reported.
func (t *Table) Clean() CleanReport {
	rep := CleanReport{RowsIn: len(t.Rows)}
	width := len(t.Header)
	rep.Kinds = make([]Kind, width)
	for c := 0; c < width; c++ {
		rep.Kinds[c] = InferKind(column(t.Rows, c))
	}
	seen := map[string]bool{}
	kept := make([][]string, 0, len(t.Rows))
	for r, row := range t.Rows {
		out := make([]string, width)
		for c := 0; c < width; c++ {
			orig := row[c]
			v := NormalizeSpace(orig)
			switch {
			case v == "":
			case rep.Kinds[c] == KindDate:
				if d, ok := ParseDate(v); ok {
					v = FormatDate(d)
				} else {
					rep.Problems = append(rep.Problems, Problem{r + 1, t.Header[c], orig, "не дата"})
				}
			case rep.Kinds[c] == KindNumber:
				if n, ok := ParseNumber(v); ok {
					v = FormatNumber(n)
				} else {
					rep.Problems = append(rep.Problems, Problem{r + 1, t.Header[c], orig, "не число"})
				}
			}
			if v != orig {
				rep.Changed++
			}
			out[c] = v
		}
		key := strings.Join(out, "\x1f")
		if seen[key] {
			rep.Duplicates++
			continue
		}
		seen[key] = true
		kept = append(kept, out)
	}
	t.Rows = kept
	rep.RowsOut = len(kept)
	return rep
}

// ColumnStats summarizes one column.
type ColumnStats struct {
	Name   string
	Kind   Kind
	Filled int
	Empty  int
	Unique int
	Min    string
	Max    string
	Top    string // most frequent value of a text column
}

// Stats computes a summary of every column.
func (t *Table) Stats() []ColumnStats {
	out := make([]ColumnStats, len(t.Header))
	for c, name := range t.Header {
		vals := column(t.Rows, c)
		st := ColumnStats{Name: name, Kind: InferKind(vals)}
		uniq := map[string]int{}
		var nmin, nmax float64
		var dmin, dmax time.Time
		haveNum, haveDate := false, false
		for _, raw := range vals {
			v := NormalizeSpace(raw)
			if v == "" {
				st.Empty++
				continue
			}
			st.Filled++
			uniq[strings.ToLower(v)]++
			switch st.Kind {
			case KindNumber:
				if n, ok := ParseNumber(v); ok {
					if !haveNum || n < nmin {
						nmin = n
					}
					if !haveNum || n > nmax {
						nmax = n
					}
					haveNum = true
				}
			case KindDate:
				if d, ok := ParseDate(v); ok {
					if !haveDate || d.Before(dmin) {
						dmin = d
					}
					if !haveDate || d.After(dmax) {
						dmax = d
					}
					haveDate = true
				}
			}
		}
		st.Unique = len(uniq)
		if haveNum {
			st.Min, st.Max = FormatNumber(nmin), FormatNumber(nmax)
		}
		if haveDate {
			st.Min, st.Max = FormatDate(dmin), FormatDate(dmax)
		}
		if st.Kind == KindText {
			best, bestN := "", 0
			for _, raw := range vals {
				v := NormalizeSpace(raw)
				if v == "" {
					continue
				}
				if n := uniq[strings.ToLower(v)]; n > bestN || (n == bestN && v < best) {
					best, bestN = v, n
				}
			}
			if bestN > 1 {
				st.Top = fmt.Sprintf("%s (%d)", best, bestN)
			}
		}
		out[c] = st
	}
	return out
}

// Group is one line of a grouped sum.
type Group struct {
	Key   string
	Rows  int
	Total float64
}

// GroupSum adds up a numeric column for each distinct value of another column.
// Keys are compared ignoring case and spacing; the first spelling seen is shown.
// Groups are sorted by total, largest first, then by key. Cells of the summed
// column that are not numbers are counted in skipped.
func (t *Table) GroupSum(by, sum string) (groups []Group, skipped int, err error) {
	bi, ok := t.Col(by)
	if !ok {
		return nil, 0, fmt.Errorf("нет колонки %q (есть: %s)", by, strings.Join(t.Header, ", "))
	}
	si, ok := t.Col(sum)
	if !ok {
		return nil, 0, fmt.Errorf("нет колонки %q (есть: %s)", sum, strings.Join(t.Header, ", "))
	}
	index := map[string]int{}
	for _, row := range t.Rows {
		n, ok := ParseNumber(row[si])
		if !ok {
			skipped++
			continue
		}
		label := NormalizeSpace(row[bi])
		if label == "" {
			label = "(пусто)"
		}
		key := strings.ToLower(label)
		i, found := index[key]
		if !found {
			i = len(groups)
			index[key] = i
			groups = append(groups, Group{Key: label})
		}
		groups[i].Rows++
		groups[i].Total += n
	}
	sort.SliceStable(groups, func(a, b int) bool {
		if groups[a].Total != groups[b].Total {
			return groups[a].Total > groups[b].Total
		}
		return groups[a].Key < groups[b].Key
	})
	return groups, skipped, nil
}
