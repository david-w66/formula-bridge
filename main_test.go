package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDialect(t *testing.T) {
	tests := []struct {
		in      string
		want    Dialect
		wantErr bool
	}{
		{"excel", Excel, false},
		{"Excel", Excel, false},
		{"calc", Calc, false},
		{"libre", Calc, false},
		{"LibreOffice", Calc, false},
		{"lotus123", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		got, err := ParseDialect(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseDialect(%q) = %q, nil, want error", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseDialect(%q) returned error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseDialect(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseOptions(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
		check   func(t *testing.T, o *options)
	}{
		{
			name: "defaults",
			args: nil,
			check: func(t *testing.T, o *options) {
				if o.from != Excel || o.to != Calc || o.inPlace || o.outPath != "" || len(o.files) != 0 {
					t.Errorf("unexpected defaults: %+v", o)
				}
			},
		},
		{
			name: "direction and files",
			args: []string{"-from", "calc", "-to", "excel", "a.txt", "b.txt"},
			check: func(t *testing.T, o *options) {
				if o.from != Calc || o.to != Excel {
					t.Errorf("dialects = %q -> %q", o.from, o.to)
				}
				if len(o.files) != 2 || o.files[0] != "a.txt" || o.files[1] != "b.txt" {
					t.Errorf("files = %v", o.files)
				}
			},
		},
		{
			name: "output file",
			args: []string{"-o", "out.txt", "in.txt"},
			check: func(t *testing.T, o *options) {
				if o.outPath != "out.txt" {
					t.Errorf("outPath = %q", o.outPath)
				}
			},
		},
		{
			name: "in-place with files",
			args: []string{"-in-place", "in.txt"},
			check: func(t *testing.T, o *options) {
				if !o.inPlace {
					t.Error("inPlace not set")
				}
			},
		},
		{name: "in-place with no files", args: []string{"-in-place"}, wantErr: "requires at least one file"},
		{name: "in-place with -o", args: []string{"-in-place", "-o", "out.txt", "in.txt"}, wantErr: "cannot be used together"},
		{name: "bad from dialect", args: []string{"-from", "lotus123"}, wantErr: "lotus123"},
		{name: "unknown flag", args: []string{"-bogus"}, wantErr: "bogus"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			o, err := parseOptions(tt.args, &stderr)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("parseOptions(%v) returned nil error, want one containing %q", tt.args, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseOptions(%v) returned error: %v", tt.args, err)
			}
			tt.check(t, o)
		})
	}
}

func TestParseOptionsHelp(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseOptions([]string{"-h"}, &stderr)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("err = %v, want flag.ErrHelp", err)
	}
	if !strings.Contains(stderr.String(), "usage: formula-bridge") {
		t.Errorf("usage text not written to stderr: %q", stderr.String())
	}
}

func TestRun(t *testing.T) {
	in := "=SUM(A1,A2)\nplain text\n\n=Sheet1!A1+1\n"
	want := "=SUM(A1;A2)\nplain text\n\n=Sheet1.A1+1\n"

	var out bytes.Buffer
	if err := run(strings.NewReader(in), &out, Excel, Calc); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if out.String() != want {
		t.Errorf("run output = %q, want %q", out.String(), want)
	}
}

func TestRunPropagatesConvertError(t *testing.T) {
	var out bytes.Buffer
	err := run(strings.NewReader("=SUM(A1,A2)\n"), &out, Dialect("bogus"), Calc)
	if err == nil {
		t.Fatal("run with an unknown dialect returned nil error, want error")
	}
}

func TestRunFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "formulas.txt")
	if err := os.WriteFile(path, []byte("=SUM(A1,A2)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runFile(path, &out, Excel, Calc); err != nil {
		t.Fatalf("runFile returned error: %v", err)
	}
	if want := "=SUM(A1;A2)\n"; out.String() != want {
		t.Errorf("runFile output = %q, want %q", out.String(), want)
	}
}

func TestRunFileMissing(t *testing.T) {
	var out bytes.Buffer
	err := runFile(filepath.Join(t.TempDir(), "does-not-exist.txt"), &out, Excel, Calc)
	if err == nil {
		t.Fatal("runFile on a missing file returned nil error, want error")
	}
}

func TestConvertInPlace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "formulas.txt")
	original := "=SUM(A1,A2)\nheader\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := convertInPlace(path, Excel, Calc); err != nil {
		t.Fatalf("convertInPlace returned error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "=SUM(A1;A2)\nheader\n"; string(got) != want {
		t.Errorf("file contents = %q, want %q", got, want)
	}
}

func TestConvertInPlaceLeavesFileUntouchedOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "formulas.txt")
	original := "=SUM(A1,A2)\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	err := convertInPlace(path, Dialect("bogus"), Calc)
	if err == nil {
		t.Fatal("convertInPlace with an unknown dialect returned nil error, want error")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Errorf("file was modified despite conversion error: got %q, want %q", got, original)
	}
}
