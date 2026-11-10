package tidy

import (
	"bytes"
	"unicode/utf8"
)

// cp1251 maps bytes 0x80..0xFF of Windows-1251 to runes.
// 0x98 is undefined in the encoding and becomes U+FFFD.
var cp1251High = []rune(
	"ЂЃ‚ѓ„…†‡€‰Љ‹ЊЌЋЏ" +
		"ђ‘’“”•–—�™љ›њќћџ" +
		" ЎўЈ¤Ґ¦§Ё©Є«¬­®Ї" +
		"°±Ііґµ¶·ё№є»јЅѕї" +
		"АБВГДЕЖЗИЙКЛМНОП" +
		"РСТУФХЦЧШЩЪЫЬЭЮЯ" +
		"абвгдежзийклмноп" +
		"рстуфхцчшщъыьэюя")

// Encoding names the detected text encoding.
type Encoding string

const (
	UTF8   Encoding = "utf-8"
	CP1251 Encoding = "windows-1251"
)

// Decode converts raw file bytes to a UTF-8 string. Input that is valid UTF-8
// (with or without a BOM) is kept as is; anything else is read as Windows-1251,
// the usual encoding of Excel "CSV" exports on Russian-language systems.
func Decode(data []byte) (string, Encoding) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(data) {
		return string(data), UTF8
	}
	out := make([]rune, 0, len(data))
	for _, b := range data {
		if b < 0x80 {
			out = append(out, rune(b))
		} else {
			out = append(out, cp1251High[b-0x80])
		}
	}
	return string(out), CP1251
}
