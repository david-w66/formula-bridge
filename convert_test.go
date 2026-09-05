package main

import "testing"

func TestConvert(t *testing.T) {
	tests := []struct {
		name    string
		formula string
		from    Dialect
		to      Dialect
		want    string
	}{
		{
			name:    "same dialect is a no-op",
			formula: "=SUM(A1,A2)",
			from:    Excel,
			to:      Excel,
			want:    "=SUM(A1,A2)",
		},
		{
			name:    "excel to calc swaps argument separator",
			formula: "=SUM(A1,A2,A3)",
			from:    Excel,
			to:      Calc,
			want:    "=SUM(A1;A2;A3)",
		},
		{
			name:    "calc to excel swaps argument separator",
			formula: "=SUM(A1;A2;A3)",
			from:    Calc,
			to:      Excel,
			want:    "=SUM(A1,A2,A3)",
		},
		{
			name:    "excel to calc rewrites sheet reference",
			formula: "=Sheet1!A1+1",
			from:    Excel,
			to:      Calc,
			want:    "=Sheet1.A1+1",
		},
		{
			name:    "calc to excel rewrites sheet reference",
			formula: "=Sheet1.A1+1",
			from:    Calc,
			to:      Excel,
			want:    "=Sheet1!A1+1",
		},
		{
			name:    "quoted sheet name survives excel to calc",
			formula: "='My Sheet'!$B$2",
			from:    Excel,
			to:      Calc,
			want:    "='My Sheet'.$B$2",
		},
		{
			name:    "quoted sheet name survives calc to excel",
			formula: "='My Sheet'.$B$2",
			from:    Calc,
			to:      Excel,
			want:    "='My Sheet'!$B$2",
		},
		{
			name:    "excel to calc rewrites 3-D range with single cell",
			formula: "=SUM(Sheet1:Sheet3!A1)",
			from:    Excel,
			to:      Calc,
			want:    "=SUM(Sheet1.A1:Sheet3.A1)",
		},
		{
			name:    "excel to calc rewrites 3-D range with cell range",
			formula: "=SUM(Sheet1:Sheet3!A1:B2)",
			from:    Excel,
			to:      Calc,
			want:    "=SUM(Sheet1.A1:Sheet3.B2)",
		},
		{
			name:    "calc to excel rewrites 3-D range with matching cells",
			formula: "=SUM(Sheet1.A1:Sheet3.A1)",
			from:    Calc,
			to:      Excel,
			want:    "=SUM(Sheet1:Sheet3!A1)",
		},
		{
			name:    "calc to excel rewrites 3-D range with differing cells",
			formula: "=SUM(Sheet1.A1:Sheet3.B2)",
			from:    Calc,
			to:      Excel,
			want:    "=SUM(Sheet1:Sheet3!A1:B2)",
		},
		{
			name:    "excel to calc renames a function",
			formula: "=STDEV.S(A1,A2)",
			from:    Excel,
			to:      Calc,
			want:    "=STDEV(A1;A2)",
		},
		{
			name:    "calc to excel renames a function",
			formula: "=STDEVP(A1;A2)",
			from:    Calc,
			to:      Excel,
			want:    "=STDEV.P(A1,A2)",
		},
		{
			name:    "function rename is case-insensitive",
			formula: "=stdev.s(A1,A2)",
			from:    Excel,
			to:      Calc,
			want:    "=STDEV(A1;A2)",
		},
		{
			name:    "excel to calc wraps bare boolean constants",
			formula: "=IF(TRUE,1,FALSE)",
			from:    Excel,
			to:      Calc,
			want:    "=IF(TRUE();1;FALSE())",
		},
		{
			name:    "excel to calc leaves existing boolean calls alone",
			formula: "=IF(TRUE(),1,0)",
			from:    Excel,
			to:      Calc,
			want:    "=IF(TRUE();1;0)",
		},
		{
			name:    "string literal separator is protected",
			formula: `=CONCATENATE(A1,"a,b",A2)`,
			from:    Excel,
			to:      Calc,
			want:    `=CONCATENATE(A1;"a,b";A2)`,
		},
		{
			name:    "string literal contents are not renamed",
			formula: `=IF(A1="TRUE",1,0)`,
			from:    Excel,
			to:      Calc,
			want:    `=IF(A1="TRUE";1;0)`,
		},
		{
			name:    "unsupported conversion returns an error",
			formula: "=SUM(A1,A2)",
			from:    Dialect("bogus"),
			to:      Calc,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Convert(tt.formula, tt.from, tt.to)
			if tt.from == Dialect("bogus") {
				if err == nil {
					t.Fatalf("Convert(%q, %s, %s) = nil error, want error", tt.formula, tt.from, tt.to)
				}
				return
			}
			if err != nil {
				t.Fatalf("Convert(%q, %s, %s) returned error: %v", tt.formula, tt.from, tt.to, err)
			}
			if got != tt.want {
				t.Errorf("Convert(%q, %s, %s) = %q, want %q", tt.formula, tt.from, tt.to, got, tt.want)
			}
		})
	}
}
