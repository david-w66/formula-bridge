package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Dialect identifies which spreadsheet application's formula syntax we're
// reading or writing.
type Dialect string

const (
	Excel Dialect = "excel"
	Calc  Dialect = "calc"
)

// ParseDialect turns a command-line flag value into a Dialect, accepting a
// couple of common spellings for Calc since people call it different
// things.
func ParseDialect(s string) (Dialect, error) {
	switch strings.ToLower(s) {
	case "excel":
		return Excel, nil
	case "calc", "libre", "libreoffice":
		return Calc, nil
	default:
		return "", fmt.Errorf("unknown dialect %q (want excel or calc)", s)
	}
}

// sheetRefExcel matches a sheet-qualified cell reference in Excel syntax,
// e.g. Sheet1!A1 or 'My Sheet'!$B$2.
var sheetRefExcel = regexp.MustCompile(`('[^']+'|[A-Za-z_][A-Za-z0-9_]*)!(\$?[A-Za-z]{1,3}\$?[0-9]+)`)

// sheetRefCalc matches the same kind of reference in LibreOffice Calc
// syntax, where the sheet name is joined to the cell with a dot instead of
// an exclamation mark. Requiring the sheet name to start with a letter or
// underscore keeps this from matching plain decimal numbers like 12.5.
var sheetRefCalc = regexp.MustCompile(`('[^']+'|[A-Za-z_][A-Za-z0-9_]*)\.(\$?[A-Za-z]{1,3}\$?[0-9]+)`)

// Convert rewrites a single formula from one dialect to another. It handles
// the two differences that break most formulas on import: the argument
// separator (comma in Excel, semicolon in Calc) and the sheet-reference
// joiner (! in Excel, . in Calc). Both are rewritten only outside of quoted
// string literals so that text arguments are left untouched.
func Convert(formula string, from, to Dialect) (string, error) {
	if from == to {
		return formula, nil
	}
	switch {
	case from == Excel && to == Calc:
		out := replaceOutsideStrings(formula, ',', ';')
		out = sheetRefExcel.ReplaceAllString(out, "$1.$2")
		return out, nil
	case from == Calc && to == Excel:
		out := replaceOutsideStrings(formula, ';', ',')
		out = sheetRefCalc.ReplaceAllString(out, "$1!$2")
		return out, nil
	default:
		return "", fmt.Errorf("unsupported conversion: %s to %s", from, to)
	}
}

// replaceOutsideStrings swaps every occurrence of old with new, except for
// occurrences that fall inside a double-quoted string literal. Both Excel
// and Calc use " to delimit text.
func replaceOutsideStrings(formula string, old, new rune) string {
	var b strings.Builder
	inString := false
	for _, r := range formula {
		if r == '"' {
			inString = !inString
			b.WriteRune(r)
			continue
		}
		if r == old && !inString {
			b.WriteRune(new)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
