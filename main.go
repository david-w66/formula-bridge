package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	fromFlag := flag.String("from", "excel", "source formula dialect: excel or calc")
	toFlag := flag.String("to", "calc", "target formula dialect: excel or calc")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [-from excel|calc] [-to excel|calc] [file]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Converts spreadsheet formulas between Excel and LibreOffice Calc syntax.\n")
		fmt.Fprintf(os.Stderr, "Reads one formula per line from the given file, or from stdin if no\n")
		fmt.Fprintf(os.Stderr, "file is given. Lines that don't start with '=' are passed through\n")
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

	var in io.Reader = os.Stdin
	if args := flag.Args(); len(args) > 0 {
		f, err := os.Open(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "formula-bridge:", err)
			os.Exit(1)
		}
		defer f.Close()
		in = f
	}

	if err := run(in, os.Stdout, from, to); err != nil {
		fmt.Fprintln(os.Stderr, "formula-bridge:", err)
		os.Exit(1)
	}
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
