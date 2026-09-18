package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	fromFlag := flag.String("from", "excel", "source formula dialect: excel or calc")
	toFlag := flag.String("to", "calc", "target formula dialect: excel or calc")
	outFlag := flag.String("o", "", "write output to this file instead of stdout")
	inPlaceFlag := flag.Bool("in-place", false, "overwrite the input file with the converted output")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [-from excel|calc] [-to excel|calc] [-o file] [-in-place] [file...]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Converts spreadsheet formulas between Excel and LibreOffice Calc syntax.\n")
		fmt.Fprintf(os.Stderr, "Reads one formula per line from the given files, in order, or from stdin\n")
		fmt.Fprintf(os.Stderr, "if none are given. Lines that don't start with '=' are passed through\n")
		fmt.Fprintf(os.Stderr, "unchanged.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	from, err := ParseDialect(*fromFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "formula-bridge:", err)
		os.Exit(1)
	}
	to, err := ParseDialect(*toFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "formula-bridge:", err)
		os.Exit(1)
	}

	args := flag.Args()
	if *inPlaceFlag {
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "formula-bridge: -in-place requires at least one file argument, not stdin")
			os.Exit(1)
		}
		if *outFlag != "" {
			fmt.Fprintln(os.Stderr, "formula-bridge: -in-place and -o cannot be used together")
			os.Exit(1)
		}
		for _, path := range args {
			if err := convertInPlace(path, from, to); err != nil {
				fmt.Fprintln(os.Stderr, "formula-bridge:", err)
				os.Exit(1)
			}
		}
		return
	}

	var out io.Writer = os.Stdout
	if *outFlag != "" {
		f, err := os.Create(*outFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, "formula-bridge:", err)
			os.Exit(1)
		}
		defer f.Close()
		out = f
	}

	if len(args) == 0 {
		if err := run(os.Stdin, out, from, to); err != nil {
			fmt.Fprintln(os.Stderr, "formula-bridge:", err)
			os.Exit(1)
		}
		return
	}

	for _, path := range args {
		if err := runFile(path, out, from, to); err != nil {
			fmt.Fprintln(os.Stderr, "formula-bridge:", err)
			os.Exit(1)
		}
	}
}

// runFile opens the file at path and runs its formulas through run, writing
// the result to w. Kept separate from run so each file's handle is closed as
// soon as it's done rather than piling up defers across an arbitrary number
// of input files.
func runFile(path string, w io.Writer, from, to Dialect) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return run(f, w, from, to)
}

// run streams formulas from r to w, converting each line that looks like a
// formula and passing everything else through untouched so a mixed file
// (headers, blank lines, plain values) survives the round trip.
func run(r io.Reader, w io.Writer, from, to Dialect) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 || line[0] != '=' {
			fmt.Fprintln(w, line)
			continue
		}
		converted, err := Convert(line, from, to)
		if err != nil {
			return err
		}
		fmt.Fprintln(w, converted)
	}
	return scanner.Err()
}

// convertInPlace converts the formulas in the file at path and overwrites it
// with the result. The output is built in memory before anything is written
// back, so a conversion error (an unbalanced quote, say) leaves the original
// file untouched instead of half-rewritten.
func convertInPlace(path string, from, to Dialect) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	err = run(f, &buf, from, to)
	f.Close()
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), info.Mode())
}
