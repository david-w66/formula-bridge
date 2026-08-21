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
$ echo '=VLOOKUP(A1,Sheet2!A:B,2,FALSE)' | formula-bridge -from excel -to calc
=VLOOKUP(A1;Sheet2.A:B;2;FALSE())
```

Going the other direction:

```
$ echo '=SUM(Sheet1.A1;Sheet1.A2)' | formula-bridge -from calc -to excel
=SUM(Sheet1!A1,Sheet1!A2)
```

Lines that don't start with `=` are passed through unchanged, so you can run
a whole exported CSV column through the tool and only the formulas get
rewritten.

## What it doesn't handle yet

- No function-name translation (a handful of functions are spelled
  differently between the two apps).
- Range references that span sheets (`Sheet1:Sheet3!A1`) aren't converted.

See the roadmap in the repo for what's planned next.

## Building

```
go build .
```

No third-party dependencies, standard library only.
