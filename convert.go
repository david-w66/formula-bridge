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

// functionRenamesExcelToCalc maps Excel function names to their Calc
// equivalent, for the small set of functions that were renamed at some
// point (mostly the Excel 2010 statistical functions that grew a dotted
// suffix) but kept an identical argument list. Functions where the newer
// name also reordered or added an argument are deliberately left out - a
// rename table isn't the place to also be reshuffling arguments, since
// getting that wrong silently produces a formula that computes the wrong
// value instead of one that fails to parse.
var functionRenamesExcelToCalc = map[string]string{
	"STDEV.S":         "STDEV",
	"STDEV.P":         "STDEVP",
	"VAR.S":           "VAR",
	"VAR.P":           "VARP",
	"MODE.SNGL":       "MODE",
	"PERCENTILE.INC":  "PERCENTILE",
	"PERCENTRANK.INC": "PERCENTRANK",
	"QUARTILE.INC":    "QUARTILE",
	"RANK.EQ":         "RANK",
	"CHISQ.TEST":      "CHITEST",
	"CHISQ.DIST.RT":   "CHIDIST",
	"CHISQ.INV.RT":    "CHIINV",
	"F.TEST":          "FTEST",
	"F.DIST.RT":       "FDIST",
	"F.INV.RT":        "FINV",
	"T.TEST":          "TTEST",
	"Z.TEST":          "ZTEST",
	"CONFIDENCE.NORM": "CONFIDENCE",
	"BETA.INV":        "BETAINV",
	"BINOM.DIST":      "BINOMDIST",
	"BINOM.INV":       "CRITBINOM",
	"EXPON.DIST":      "EXPONDIST",
	"GAMMA.DIST":      "GAMMADIST",
	"GAMMA.INV":       "GAMMAINV",
	"LOGNORM.INV":     "LOGINV",
	"NORM.DIST":       "NORMDIST",
	"NORM.INV":        "NORMINV",
	"NORM.S.INV":      "NORMSINV",
	"POISSON.DIST":    "POISSON",
	"WEIBULL.DIST":    "WEIBULL",
}

// functionRenamesCalcToExcel is the reverse of functionRenamesExcelToCalc.
// The mapping is one-to-one in both directions, so it's built by inversion
// rather than kept as a second table that could drift out of sync.
var functionRenamesCalcToExcel = invertStringMap(functionRenamesExcelToCalc)

func invertStringMap(m map[string]string) map[string]string {
	inv := make(map[string]string, len(m))
	for k, v := range m {
		inv[v] = k
	}
	return inv
}

// functionCallName matches a bare identifier immediately followed by an
// opening parenthesis - i.e. the name half of a function call.
var functionCallName = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9_.]*\(`)

// renameFunctions rewrites any function call in s whose name appears in
// renames to use the target dialect's name instead. Lookups are
// case-insensitive (formulas are conventionally uppercase, but this
// tolerates lowercase input); a matched name is normalized to the
// renamed table's casing, and anything that doesn't match is left exactly
// as written.
func renameFunctions(s string, renames map[string]string) string {
	return functionCallName.ReplaceAllStringFunc(s, func(tok string) string {
		name := strings.ToUpper(tok[:len(tok)-1])
		if renamed, ok := renames[name]; ok {
			return renamed + "("
		}
		return tok
	})
}

// bareBoolean matches the words TRUE and FALSE on their own.
var bareBoolean = regexp.MustCompile(`\b(TRUE|FALSE)\b`)

// addBooleanParens rewrites Excel's bare TRUE/FALSE constants into Calc's
// TRUE()/FALSE() function calls. Excel accepts both forms, but Calc's
// formula grammar only defines TRUE() and FALSE() as functions, so a bare
// TRUE surviving a straight copy fails to parse in Calc. Occurrences
// already followed by '(' are left alone so this doesn't double up on a
// formula that already uses the function-call form.
func addBooleanParens(s string) string {
	var b strings.Builder
	last := 0
	for _, loc := range bareBoolean.FindAllStringIndex(s, -1) {
		end := loc[1]
		b.WriteString(s[last:end])
		if end >= len(s) || s[end] != '(' {
			b.WriteString("()")
		}
		last = end
	}
	b.WriteString(s[last:])
	return b.String()
}

// Convert rewrites a single formula from one dialect to another. It handles
// the differences that break most formulas on import: the argument
// separator (comma in Excel, semicolon in Calc), the sheet-reference joiner
// (! in Excel, . in Calc), a handful of renamed functions, and Excel's bare
// TRUE/FALSE constants. All of these are rewritten only outside of quoted
// string literals and quoted sheet names, so text arguments and sheet names
// that happen to contain a comma, semicolon, or the word TRUE are left
// untouched.
func Convert(formula string, from, to Dialect) (string, error) {
	if from == to {
		return formula, nil
	}
	switch {
	case from == Excel && to == Calc:
		out := replaceOutsideStrings(formula, ',', ';')
		out = sheetRefExcel.ReplaceAllString(out, "$1.$2")
		out = rewriteOutsideStrings(out, func(s string) string {
			return addBooleanParens(renameFunctions(s, functionRenamesExcelToCalc))
		})
		return out, nil
	case from == Calc && to == Excel:
		out := replaceOutsideStrings(formula, ';', ',')
		out = sheetRefCalc.ReplaceAllString(out, "$1!$2")
		out = rewriteOutsideStrings(out, func(s string) string {
			return renameFunctions(s, functionRenamesCalcToExcel)
		})
		return out, nil
	default:
		return "", fmt.Errorf("unsupported conversion: %s to %s", from, to)
	}
}

// replaceOutsideStrings swaps every occurrence of old with new, except for
// occurrences that fall inside a double-quoted string literal or a
// single-quoted sheet name. Both Excel and Calc use " to delimit text and '
// to quote a sheet name that contains spaces or other punctuation, and a
// sheet name is free to contain a comma or semicolon, so those need the same
// protection as text literals do. A literal apostrophe inside a sheet name
// is written as a doubled '', which this treats as closing and immediately
// reopening the quote - harmless, since there's nothing between them to
// swap.
func replaceOutsideStrings(formula string, old, new rune) string {
	var b strings.Builder
	inString := false
	inSheetName := false
	for _, r := range formula {
		switch {
		case r == '"' && !inSheetName:
			inString = !inString
			b.WriteRune(r)
			continue
		case r == '\'' && !inString:
			inSheetName = !inSheetName
			b.WriteRune(r)
			continue
		}
		if r == old && !inString && !inSheetName {
			b.WriteRune(new)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// rewriteOutsideStrings applies rewrite to the parts of formula that fall
// outside any double-quoted string literal or single-quoted sheet name,
// leaving the quoted parts (delimiters included) untouched. This lets a
// regex-based rewrite - like a function rename or the TRUE/FALSE fix-up -
// run without corrupting text that happens to contain a function name or
// the word TRUE.
func rewriteOutsideStrings(formula string, rewrite func(string) string) string {
	var out strings.Builder
	var plain strings.Builder
	inString := false
	inSheetName := false
	flush := func() {
		out.WriteString(rewrite(plain.String()))
		plain.Reset()
	}
	for _, r := range formula {
		switch {
		case r == '"' && !inSheetName:
			if !inString {
				flush()
			}
			inString = !inString
			out.WriteRune(r)
			continue
		case r == '\'' && !inString:
			if !inSheetName {
				flush()
			}
			inSheetName = !inSheetName
			out.WriteRune(r)
			continue
		}
		if inString || inSheetName {
			out.WriteRune(r)
		} else {
			plain.WriteRune(r)
		}
	}
	flush()
	return out.String()
}
