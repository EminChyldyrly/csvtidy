// csvtidy приводит в порядок «грязные» CSV-выгрузки: кодировку, разделитель,
// числа, даты, пробелы и дубликаты, и считает простые сводки.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/EminChyldyrly/csvtidy/tidy"
)

const version = "0.1.0"

const usage = `csvtidy — чистка и быстрая сводка CSV-выгрузок

Команды:
  clean  файл.csv [-o out.csv] [-delim ,]     привести данные в порядок
  stats  файл.csv                              описать колонки
  sum    файл.csv -by КОЛОНКА -sum КОЛОНКА     сумма по группам [-csv]
  version                                      показать версию

Файл "-" означает стандартный ввод. Кодировка (UTF-8 или Windows-1251)
и разделитель (; , табуляция |) определяются сами.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run выполняет команду и возвращает код выхода: 0 — успех, 1 — ошибка данных
// или файла, 2 — неверное использование.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "clean":
		return cmdClean(args[1:], stdin, stdout, stderr)
	case "stats":
		return cmdStats(args[1:], stdin, stdout, stderr)
	case "sum":
		return cmdSum(args[1:], stdin, stdout, stderr)
	case "version", "-v", "--version":
		fmt.Fprintln(stdout, "csvtidy", version)
		return 0
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	}
	fmt.Fprintf(stderr, "неизвестная команда %q\n\n%s", args[0], usage)
	return 2
}

// parseArgs разбирает флаги и один позиционный аргумент (файл) в любом порядке.
func parseArgs(fs *flag.FlagSet, args []string, stderr io.Writer) (string, bool) {
	fs.SetOutput(stderr)
	var file string
	for {
		if err := fs.Parse(args); err != nil {
			return "", false
		}
		rest := fs.Args()
		if len(rest) == 0 {
			break
		}
		if file != "" {
			fmt.Fprintf(stderr, "лишний аргумент %q\n", rest[0])
			return "", false
		}
		file, args = rest[0], rest[1:]
	}
	if file == "" {
		fmt.Fprintln(stderr, "не указан файл")
		return "", false
	}
	return file, true
}

func load(file string, stdin io.Reader, stderr io.Writer) (*tidy.Table, bool) {
	var data []byte
	var err error
	if file == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(file)
	}
	if err != nil {
		fmt.Fprintln(stderr, "ошибка:", err)
		return nil, false
	}
	t, err := tidy.Parse(data)
	if err != nil {
		fmt.Fprintln(stderr, "ошибка:", err)
		return nil, false
	}
	return t, true
}

func cmdClean(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	out := fs.String("o", "", "куда записать результат (по умолчанию — стандартный вывод)")
	delim := fs.String("delim", ",", "разделитель в результате")
	file, ok := parseArgs(fs, args, stderr)
	if !ok {
		return 2
	}
	if utf8.RuneCountInString(*delim) != 1 {
		fmt.Fprintln(stderr, "ошибка: -delim должен быть одним символом")
		return 2
	}
	t, ok := load(file, stdin, stderr)
	if !ok {
		return 1
	}
	enc, d := t.Encoding, t.Delimiter
	rep := t.Clean()

	var w io.Writer = stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintln(stderr, "ошибка:", err)
			return 1
		}
		defer f.Close()
		w = f
	}
	r, _ := utf8.DecodeRuneInString(*delim)
	if err := t.Write(w, r); err != nil {
		fmt.Fprintln(stderr, "ошибка записи:", err)
		return 1
	}

	fmt.Fprintf(stderr, "кодировка: %s, разделитель: %q\n", enc, d)
	fmt.Fprintf(stderr, "строк: %d → %d (дубликатов убрано: %d), исправлено ячеек: %d\n",
		rep.RowsIn, rep.RowsOut, rep.Duplicates, rep.Changed)
	for i, k := range rep.Kinds {
		fmt.Fprintf(stderr, "  %-24s %s\n", t.Header[i], k)
	}
	if n := len(rep.Problems); n > 0 {
		fmt.Fprintf(stderr, "проблемные ячейки (%d), оставлены как есть:\n", n)
		for i, p := range rep.Problems {
			if i == 20 {
				fmt.Fprintf(stderr, "  … и ещё %d\n", n-20)
				break
			}
			fmt.Fprintf(stderr, "  строка %d, %q: %q — %s\n", p.Row, p.Column, p.Value, p.Reason)
		}
	}
	return 0
}

func cmdStats(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	file, ok := parseArgs(fs, args, stderr)
	if !ok {
		return 2
	}
	t, ok := load(file, stdin, stderr)
	if !ok {
		return 1
	}
	fmt.Fprintf(stdout, "строк: %d, колонок: %d, кодировка: %s, разделитель: %q\n\n",
		len(t.Rows), len(t.Header), t.Encoding, t.Delimiter)
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "колонка\tтип\tзаполнено\tпусто\tразных\tмин\tмакс\tчаще всего")
	for _, s := range t.Stats() {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%s\t%s\t%s\n",
			s.Name, s.Kind, s.Filled, s.Empty, s.Unique, s.Min, s.Max, s.Top)
	}
	tw.Flush()
	return 0
}

func cmdSum(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sum", flag.ContinueOnError)
	by := fs.String("by", "", "колонка для группировки")
	sum := fs.String("sum", "", "числовая колонка для суммирования")
	asCSV := fs.Bool("csv", false, "вывести CSV вместо таблицы")
	file, ok := parseArgs(fs, args, stderr)
	if !ok {
		return 2
	}
	if *by == "" || *sum == "" {
		fmt.Fprintln(stderr, "нужны оба флага: -by и -sum")
		return 2
	}
	t, ok := load(file, stdin, stderr)
	if !ok {
		return 1
	}
	groups, skipped, err := t.GroupSum(*by, *sum)
	if err != nil {
		fmt.Fprintln(stderr, "ошибка:", err)
		return 1
	}
	var total float64
	var rows int
	for _, g := range groups {
		total += g.Total
		rows += g.Rows
	}
	if *asCSV {
		fmt.Fprintf(stdout, "%s,строк,%s\n", csvEscape(*by), csvEscape(*sum))
		for _, g := range groups {
			fmt.Fprintf(stdout, "%s,%d,%s\n", csvEscape(g.Key), g.Rows, tidy.FormatNumber(round2(g.Total)))
		}
	} else {
		tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', tabwriter.AlignRight|tabwriter.Debug)
		fmt.Fprintf(tw, "%s\tстрок\t%s\t\n", *by, *sum)
		for _, g := range groups {
			fmt.Fprintf(tw, "%s\t%d\t%s\t\n", g.Key, g.Rows, tidy.FormatNumber(round2(g.Total)))
		}
		fmt.Fprintf(tw, "%s\t%d\t%s\t\n", "ИТОГО", rows, tidy.FormatNumber(round2(total)))
		tw.Flush()
	}
	if skipped > 0 {
		fmt.Fprintf(stderr, "пропущено строк без числа в колонке %q: %d\n", *sum, skipped)
	}
	return 0
}

func round2(v float64) float64 {
	if v < 0 {
		return -float64(int64(-v*100+0.5)) / 100
	}
	return float64(int64(v*100+0.5)) / 100
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}
