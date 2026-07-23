# NEM12 → meter_readings SQL Generator

A command-line tool that reads a NEM12-format meter data file and generates
`INSERT` statements for the `meter_readings` table. The tool only **generates**
SQL — it does not connect to or write into any database.

## What it does

- Parses NEM12 records (`100`, `200`, `300`, `500`, `900`)
- For each `200` block (an NMI + interval length), expands every `300` record
  (a day of readings) into individual interval readings
- Writes batched `INSERT INTO meter_readings (...) VALUES (...)` statements
  to a `.sql` file
- Streams the input file line-by-line, so memory use stays low regardless of
  file size

## Requirements

- Go 1.21+ (no other tools or services required)

## Quick start

```bash
make build
make run
```

This compiles the binary and runs it against the default input/output paths:

```bash
./bin/nem12parser -input energymeter.csv -output output.sql
```

Result: `output.sql` is created (or overwritten) in the project root,
containing the generated `INSERT` statements.

## Usage

```bash
make run INPUT=path/to/your-file.csv OUTPUT=path/to/result.sql
```

or run the binary directly after building:

```bash
./bin/nem12parser -input path/to/your-file.csv -output path/to/result.sql
```

| Flag | Default | Description |
|---|---|---|
| `-input` | `energymeter.csv` | Path to the NEM12 input file |
| `-output` | `output.sql` | Path to write the generated SQL to |


## Output format

Generated SQL looks like:

```sql
INSERT INTO meter_readings (id, nmi, "timestamp", consumption) VALUES
  ('a1b2c3d4-...', 'NEM1201009', '2005-03-01 00:30:00', 0),
  ('a1b2c3d4-...', 'NEM1201009', '2005-03-01 01:00:00', 0),
  ...
ON CONFLICT ON CONSTRAINT meter_readings_unique_consumption DO NOTHING;
```

- Rows are batched (1000 per statement) rather than one statement per row.
- `ON CONFLICT ... DO NOTHING` makes the output idempotent — running it more
  than once against the same table won't error or duplicate rows.

## Write-up

See [writeup.md](./writeup.md) for the assessment's Q1–Q3 answers (technology
rationale, design rationale, and what would be done differently with more time).