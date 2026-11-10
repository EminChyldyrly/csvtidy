package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCmd(t *testing.T, stdin string, args ...string) (code int, out, errOut string) {
	t.Helper()
	var o, e bytes.Buffer
	code = run(args, strings.NewReader(stdin), &o, &e)
	return code, o.String(), e.String()
}

const messy = "Менеджер;Дата;Сумма\nАнна;05.03.2024;1 000,50\nАнна;5.3.24;1 000,50\nБорис;06.03.2024;2 000\n"

func TestCleanFromStdin(t *testing.T) {
	code, out, errOut := runCmd(t, messy, "clean", "-")
	if code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	want := "Менеджер,Дата,Сумма\nАнна,2024-03-05,1000.5\nБорис,2024-03-06,2000\n"
	if out != want {
		t.Errorf("out = %q, want %q", out, want)
	}
	if !strings.Contains(errOut, "дубликатов убрано: 1") {
		t.Errorf("report missing duplicates: %s", errOut)
	}
}

func TestCleanToFileWithDelimiter(t *testing.T) {
	dir := t.TempDir()
	in, outFile := filepath.Join(dir, "in.csv"), filepath.Join(dir, "out.csv")
	if err := os.WriteFile(in, []byte(messy), 0o644); err != nil {
		t.Fatal(err)
	}
	// flags may come after the file name
	code, _, errOut := runCmd(t, "", "clean", in, "-o", outFile, "-delim", ";")
	if code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	got, _ := os.ReadFile(outFile)
	if string(got) != "Менеджер;Дата;Сумма\nАнна;2024-03-05;1000.5\nБорис;2024-03-06;2000\n" {
		t.Errorf("file = %q", got)
	}
}

func TestSum(t *testing.T) {
	code, out, errOut := runCmd(t, messy, "sum", "-", "-by", "менеджер", "-sum", "Сумма", "-csv")
	if code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	if out != "менеджер,строк,Сумма\nБорис,1,2000\nАнна,2,2001\n" && out != "менеджер,строк,Сумма\nАнна,2,2001\nБорис,1,2000\n" {
		t.Errorf("out = %q", out)
	}
	if !strings.HasPrefix(out, "менеджер,строк,Сумма\nАнна,2,2001\n") {
		t.Errorf("largest group must come first: %q", out)
	}
}

func TestStatsOutput(t *testing.T) {
	code, out, _ := runCmd(t, messy, "stats", "-")
	if code != 0 || !strings.Contains(out, "строк: 3, колонок: 3") || !strings.Contains(out, "дата") {
		t.Errorf("code %d, out %q", code, out)
	}
}

func TestExitCodes(t *testing.T) {
	cases := []struct {
		args []string
		in   string
		want int
	}{
		{nil, "", 2},
		{[]string{"boom"}, "", 2},
		{[]string{"clean"}, "", 2},
		{[]string{"clean", "a", "b"}, "", 2},
		{[]string{"clean", "-", "-delim", ";;"}, messy, 2},
		{[]string{"sum", "-"}, messy, 2},
		{[]string{"clean", "/no/such/file.csv"}, "", 1},
		{[]string{"clean", "-"}, "   \n", 1},
		{[]string{"sum", "-", "-by", "нет", "-sum", "Сумма"}, messy, 1},
		{[]string{"version"}, "", 0},
		{[]string{"help"}, "", 0},
	}
	for _, c := range cases {
		if code, _, _ := runCmd(t, c.in, c.args...); code != c.want {
			t.Errorf("run(%v) = %d, want %d", c.args, code, c.want)
		}
	}
}

func TestRound2(t *testing.T) {
	for in, want := range map[float64]float64{2.5: 2.5, 2.499: 2.5, -1.239: -1.24, 0: 0, 1234.5678: 1234.57} {
		if got := round2(in); got != want {
			t.Errorf("round2(%v) = %v, want %v", in, got, want)
		}
	}
}
