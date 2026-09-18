# formula-bridge

Excel and LibreOffice Calc write the same formula differently. Move a sheet
from one to the other and formulas that reference other sheets, or take more
than one argument, come back broken:

```
Excel:  =SUM(Sheet1!A1,Sheet1!A2)
Calc:   =SUM(Sheet1.A1;Sheet1.A2)
```

The argument separator flips between `,` and `;`, and the sheet-reference
joiner flips between `!` and `.`. `formula-bridge` rewrites formulas from one
dialect to the other so a copy-pasted or exported formula still works after
the move.

## Usage

Convert a file of formulas, one per line:

```
$ formula-bridge -from excel -to calc formulas.txt
=SUM(Sheet1.A1;Sheet1.A2)
=IF(A1>0;"ok";"check")
```

Or pipe formulas in on stdin, which is the main way this is meant to be used
(e.g. as a filter in an export/import pipeline):

```
$ echo '=IF(STDEV.S(A1:A10)>0,TRUE,FALSE)' | formula-bridge -from excel -to calc
=IF(STDEV(A1:A10)>0;TRUE();FALSE())
```

Going the other direction:

```
$ echo '=SUM(Sheet1.A1;Sheet1.A2)' | formula-bridge -from calc -to excel
=SUM(Sheet1!A1,Sheet1!A2)
```

References that span a range of sheets (a 3-D reference) are also converted,
even though the two dialects structure them differently - Excel keeps the
sheet range together and appends the cell once, while Calc dots each
boundary sheet to its own cell:

```
$ echo '=SUM(Sheet1:Sheet3!A1)' | formula-bridge -from excel -to calc
=SUM(Sheet1.A1:Sheet3.A1)
```

Lines that don't start with `=` are passed through unchanged, so you can run
a whole exported CSV column through the tool and only the formulas get
rewritten.

Use `-o` to write the result to a file instead of stdout:

```
$ formula-bridge -from excel -to calc -o formulas.calc.txt formulas.txt
```

Give more than one file and they're converted in order, one after another,
into the same output:

```
$ formula-bridge -from excel -to calc -o formulas.calc.txt sheet1.txt sheet2.txt
```

`-in-place` also accepts multiple files, overwriting each one with its own
converted contents:

```
$ formula-bridge -from excel -to calc -in-place sheet1.txt sheet2.txt
```

## What it doesn't handle yet

- Function-name translation only covers a fixed table of same-signature
  renames (mostly the Excel 2010 statistical functions and their older Calc
  equivalents, plus Excel's bare `TRUE`/`FALSE`). Functions whose argument
  list also changed between the two apps aren't touched, since guessing at
  an argument reorder is worse than leaving the formula alone.

See the roadmap in the repo for what's planned next.

## Building

```
go build .
```

No third-party dependencies, standard library only.
