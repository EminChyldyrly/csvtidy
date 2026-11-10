# csvtidy

*[Русская версия](README.md)*

A command-line tool in Go that tidies up messy CSV exports and computes quick summaries. A single binary, **standard library only**, no dependencies. Sample data and messages are in Russian.

Exports from accounting systems, Excel and CRMs tend to look alike: a Windows-1251 file, a `;` delimiter, dates in three formats, amounts like `1 234,50`, `2 500 руб.` and `1,234.50`, stray spaces and repeated rows. `csvtidy` fixes all of this in one run and tells you what it did and which cells it could not parse.

## What it does

| Command | What it does |
|---|---|
| `clean` | detects the encoding (UTF-8 or Windows-1251) and the delimiter (`;` `,` tab `\|`); trims extra spaces; writes numbers as `1234.5` and dates as `2024-03-05`; removes rows that became exact duplicates; writes UTF-8 |
| `stats` | describes the columns: type (number, date, text), filled and empty counts, distinct values, minimum and maximum, the most frequent value |
| `sum` | adds up a numeric column for each value of another one: "revenue per manager", "total per category" |

## Quick start

Go 1.22 or newer is required.

```bash
git clone https://github.com/EminChyldyrly/csvtidy
cd csvtidy
go build -o csvtidy .

./csvtidy clean docs/example_orders.csv -o clean.csv
./csvtidy stats docs/example_orders.csv
./csvtidy sum clean.csv -by Категория -sum Сумма
```

The file name `-` means standard input, so commands chain in a pipeline: `csvtidy clean - < raw.csv | csvtidy sum - -by Менеджер -sum Сумма`.

### Example

The input [`docs/example_orders.csv`](docs/example_orders.csv) is a Windows-1251 file with `;` as the delimiter and 32 rows, two of them exact copies, with dates in four formats and amounts written in different ways. `csvtidy clean docs/example_orders.csv -o clean.csv` reports to standard error:

```
кодировка: windows-1251, разделитель: ';'
строк: 32 → 30 (дубликатов убрано: 2), исправлено ячеек: 63
  ...
проблемные ячейки (2), оставлены как есть:
  строка 8, "Сумма": "договорная" — не число
  строка 10, "Дата": "31.02.2024" — не дата
```

That is: encoding windows-1251, delimiter `;`; 32 rows became 30 (2 duplicates removed), 63 cells rewritten; two cells that did not fit their column ("договорная" in the amount column and the non-existent date 31.02.2024) were left untouched and listed. The result is in [`docs/example_clean.csv`](docs/example_clean.csv):

```
Номер,Дата,Менеджер,Категория,Сумма,Кол-во
A-1001,2024-05-15,Петров С.,техника,15400,3
A-1002,2024-04-04,Козлов Д.,Канцелярия,2500,7
A-1003,2024-06-15,Козлов Д.,Канцелярия,320,1
```

## How numbers and dates are read

- **Numbers:** `1234`, `1 234,50`, `1,234.50`, `1.234,50`, `2 500 ₽`, `2500 руб.`, `$15`, `-12`, `(12)` (negative), `15%`. If both `.` and `,` are present, the last one is the decimal mark. A single separator that appears once is decimal (`1,5` and `1.5` both mean 1.5). The same separator repeated is a thousands separator, with groups of exactly three digits (`1,234,567`), so `1.2.3` is not a number.
- **Dates:** the day comes first, so `03.04.2024` is 3 April. Supported: `05.03.2024`, `5.3.24`, `05/03/2024`, `05-03-2024`, `2024-03-05`, `2024.03.05`; a time after the date is dropped. Non-existent dates (`31.02.2024`) and years outside 1900–2100 are rejected, so plain numbers are not mistaken for dates.
- **Column type** is inferred from the values: if at least 80 % of the filled cells parse as dates (or as numbers), the column gets that type. Cells that do not fit are left as they were and appear in the problem report, so nothing disappears silently.

## Exit codes

`0` success; `1` file or data error (missing file, empty file, no such column); `2` wrong usage (missing argument, unknown command).

## How quality is verified

`go test ./...` runs 20 tests (many are table-driven with dozens of cases); coverage is about 97 % for the `tidy` package and 82 % for the program as a whole.

- Number and date parsing is checked against lists of valid and invalid inputs, including edge cases: a leap day, `31.02`, the year 1800, `1.2.3`, `1,2,3.4`, a non-breaking space and a Unicode minus.
- The Windows-1251 table is checked for length (128 entries) and for the whole `А–я` alphabet.
- Running `clean` on its own output changes nothing (idempotence).
- Commands are run end to end through the `run` function: standard input, writing to a file, flags after the file name, exit codes.

I also broke the source by hand in several places (the column-type threshold, sort order, the duplicate check, the date format, the decimal-mark choice and so on) to confirm that the tests notice. That was a selective check, not an automated mutation-testing run.

```bash
go test ./...
go test -cover ./...
```

## Structure

```
main.go          clean / stats / sum commands, flag parsing, exit codes
tidy/decode.go   encoding detection and Windows-1251 decoding
tidy/parse.go    ParseNumber, ParseDate, whitespace normalization
tidy/table.go    reading and writing CSV, delimiter detection, headers
tidy/ops.go      column types, Clean, Stats, GroupSum
docs/            a messy sample export and the cleaned result
```

## Limitations

- Only UTF-8 and Windows-1251 are detected; others (KOI8-R, UTF-16) are not.
- The whole file is read into memory, so the tool is not meant for files of hundreds of megabytes.
- Text spellings are not merged: "Техника" and "техника" stay different values (`sum` adds them together because it ignores case).
- Dates with the month in words ("5 марта 2024") are not parsed.
- Only CSV is supported, not `.xlsx`.

## License

MIT, see [LICENSE](LICENSE).
