package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// flagParseError marks errors the flag package has already reported to
// stderr, so main doesn't print them a second time.
type flagParseError struct{ err error }

func (e flagParseError) Error() string { return e.err.Error() }
func (e flagParseError) Unwrap() error { return e.err }

// options holds the validated command line.
type options struct {
	from, to Dialect
	outPath  string
	inPlace  bool
	files    []string
}

// parseOptions parses and validates args (without the program name). It uses
// its own FlagSet and returns errors instead of exiting so the flag
// combinations can be tested; usage and flag errors are written to stderr.
func parseOptions(args []string, stderr io.Writer) (*options, error) {
	fs := flag.NewFlagSet("formula-bridge", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fromFlag := fs.String("from", "excel", "source formula dialect: excel or calc")
	toFlag := fs.String("to", "calc", "target formula dialect: excel or calc")
	outFlag := fs.String("o", "", "write output to this file instead of stdout")
	inPlaceFlag := fs.Bool("in-place", false, "overwrite the input file with the converted output")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: formula-bridge [-from excel|calc] [-to excel|calc] [-o file] [-in-place] [file...]\n\n")
		fmt.Fprintf(stderr, "Converts spreadsheet formulas between Excel and LibreOffice Calc syntax.\n")
		fmt.Fprintf(stderr, "Reads one formula per line from the given files, in order, or from stdin\n")
		fmt.Fprintf(stderr, "if none are given. Lines that don't start with '=' are passed through\n")
		fmt.Fprintf(stderr, "unchanged.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return nil, flagParseError{err}
	}

	from, err := ParseDialect(*fromFlag)
	if err != nil {
		return nil, err
	}
	to, err := ParseDialect(*toFlag)
	if err != nil {
		return nil, err
	}

	opts := &options{from: from, to: to, outPath: *outFlag, inPlace: *inPlaceFlag, files: fs.Args()}
	if opts.inPlace {
		if len(opts.files) == 0 {
			return nil, errors.New("-in-place requires at least one file argument, not stdin")
		}
		if opts.outPath != "" {
			return nil, errors.New("-in-place and -o cannot be used together")
		}
	}
	return opts, nil
}

func main() {
	opts, err := parseOptions(os.Args[1:], os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		// The flag package has already printed its own message for parse errors.
		var pe flagParseError
		if !errors.As(err, &pe) {
			fmt.Fprintln(os.Stderr, "formula-bridge:", err)
		}
		os.Exit(1)
	}
	from, to, args := opts.from, opts.to, opts.files

	if opts.inPlace {
		for _, path := range args {
			if err := convertInPlace(path, from, to); err != nil {
				fmt.Fprintln(os.Stderr, "formula-bridge:", err)
				os.Exit(1)
			}
		}
		return
	}

	var out io.Writer = os.Stdout
	if opts.outPath != "" {
		f, err := os.Create(opts.outPath)
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
