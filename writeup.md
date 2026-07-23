# Write-up

## Q1. What is the rationale for the technologies you have decided to use?

**Go**, standard library only, was chosen as the implementation language:

- **Strong I/O streaming primitives.** `bufio.Scanner` reads a file line-by-line without loading it into memory, which maps directly onto the "handle files of very large sizes" requirement. NEM12 is a line-oriented format, so streaming parsing is a natural fit — the file is never held in memory as a whole.
- **No CSV parser/ORM needed.** NEM12 is comma-separated but not standard CSV (variable field counts per record type, no quoting/escaping rules to worry about). `strings.Split` per line is sufficient and keeps the parsing logic transparent and easy to review.
- **No ORM, no database driver.** Since the deliverable is *generating* INSERT statements, not executing them, there's no runtime dependency on a database at all. The output is a plain `.sql` file — a reviewer can run the tool and read the result without installing or configuring anything beyond Go itself.
- **`google/uuid`** is the one small external dependency, used only to generate `id` values matching the `id uuid default gen_random_uuid()` column shape in the generated INSERTs.

The end result has effectively zero setup cost: `go run main.go -input file.csv -output output.sql` produces the deliverable directly, with no database, Docker, or infrastructure involved.

## Q2. What would you have done differently if you had more time?

- **Actually applying the generated SQL**, as a clearly separate, optional concern — e.g. a small follow-up script or `-apply` flag that connects to Postgres and runs the generated statements.
- **Structured tests.** With more time I'd add table-driven unit tests covering: standard files, files with missing optional trailing fields (as seen in the sample), malformed rows (wrong field count, bad interval length), multiple NMI blocks in one file, and very large synthetic files to benchmark memory/throughput and confirm the generated SQL is syntactically correct at scale.
- **CLI ergonomics.** Progress reporting for large files (rows/sec, ETA) and a `--dry-run`/validate-only mode that checks the file without writing output.
- **Checkpointing/resumability for very large files.** For truly large files, a failure partway through (bad row, process crash, disk full) currently means re-running from the start. I'd add a checkpoint mechanism — e.g. Redis storing the last successfully processed line number (or NMI block) per input file — so the tool could detect a previous partial run and resume from that point rather than reprocessing everything. Practically this would mean: on each completed block, write `checkpoint:<file-hash> = <line-number>` to Redis; on startup, check for an existing checkpoint and seek/skip to that line before continuing to write to the output SQL file (in append mode rather than truncate). This trades a small amount of complexity and an extra dependency for much cheaper recovery on multi-gigabyte files where reprocessing from scratch is expensive. It's the kind of addition I'd only reach for once file sizes are large enough that restart cost is a real operational concern — for the scope of this assessment, fail-fast-and-rerun is simpler and sufficient.

## Q3. What is the rationale for the design choices that you have made?

- **Streaming, not batch-loading the whole file.** The parser reads and processes one line at a time via `bufio.Scanner`, emitting a completed `MeterData` block (one NMI's readings) as soon as the next `200` record or EOF is hit. Memory usage stays bounded to a single block's readings rather than growing with file size — this is the core mechanism satisfying the "very large files" requirement, more so than any buffer-size tuning.
- **Callback-based block emission (`onBlock func(MeterData) error`)** rather than returning `[]MeterData` from the parser. This lets the caller write out each block's SQL immediately and discard it, instead of accumulating the entire parsed file (and therefore the entire output) in memory before writing anything.
- **Fail-fast validation on malformed input**, rather than best-effort/silent handling:
  - Interval length must evenly divide 1440 minutes — an invalid value (e.g. `0` or a non-divisor) would otherwise silently produce the wrong number of intervals per day via integer division truncation, corrupting every derived timestamp in the generated SQL without any visible error.
  - Field-count checks on `300` records before indexing into them — without this, a truncated/malformed row would cause a Go runtime panic (`index out of range`) mid-file, potentially after a large amount of correct output was already written, rather than a clean, line-numbered error.
  Both are deliberately strict: since the output is SQL destined for a table with billing-adjacent data and a uniqueness constraint on `(nmi, timestamp)`, a loud early failure is safer than silently generating a statement with wrong values.
- **Batched INSERT statements with `ON CONFLICT DO NOTHING`.** Rows are grouped into multi-row `INSERT` statements (1000 rows per statement) rather than one statement per row, which keeps the generated file compact and, if the SQL is later run, keeps round-trips low. Each statement includes `ON CONFLICT ON CONSTRAINT meter_readings_unique_consumption DO NOTHING`, so the output is idempotent by construction — safe to run (or re-run) against the table without erroring even if the same file is processed twice or files overlap.