---
name: ywiki
description: Read and write Yandex Wiki pages, search the wiki, and manage comments, attachments, and dynamic tables from the command line. Use when a task involves Yandex Wiki content - reading a page, publishing notes or a report to the wiki, searching internal documentation, commenting on a page, or reading and editing a table (grid).
---

# ywiki

Command-line client for Yandex Wiki. Every command takes a page as either a
slug (`users/me/notes`) or a numeric page ID, and supports machine-readable
output.

## Before starting

Check that credentials work:

```bash
ywiki auth status --json valid --jq '.valid'
```

If this is not `true`, stop and ask the user to run `ywiki auth login`. Never
ask for or handle their token directly.

## Output contract

- `--json <fields>` returns an object (single result) or array (list) with only
  those fields. A `--json` with no field names prints the valid fields.
- `--jq <expr>` filters the full response. List commands wrap results as
  `{"items": [...], "pagination": {"cursor": "...", "hasMore": true}}`, so
  `--jq '.items[].slug'` reads the results and `--jq '.pagination.cursor'`
  reads the continuation cursor.
- `--quiet` prints identifiers only, one per line, for shell loops.
- Errors go to stderr with an exit code: 1 invalid input, 3 auth, 4 not found,
  5 rate limited, 6 conflict (the page changed since it was read).

## Reading

```bash
# Page body only, nothing else - use this to read a page into context
ywiki page get users/me/notes --content

# Metadata as JSON
ywiki page get users/me/notes --json id,slug,title,modifiedAt,author

# Direct children, or the whole subtree
ywiki page list users/me --json id,slug
ywiki page list users/me --recursive --json slug

# Full-text search
ywiki search "deployment runbook" --limit 20 --json slug,title,snippet
```

Listings are cursor-paginated. Follow the cursor rather than assuming the first
page is everything:

```bash
cursor=$(ywiki page list users/me --jq '.pagination.cursor')
ywiki page list users/me --cursor "$cursor" --json slug
```

## Writing

Prefer `append` over `edit` when adding to a page: `edit` replaces the whole
body and will silently discard anything written by someone else since the read,
while `append` cannot.

```bash
# Add to a page
ywiki page append users/me/log --body "Deployed v1.2.0"
cat entry.md | ywiki page append users/me/log --body-file - --location top

# Create a page
ywiki page create users/me/report --title "Q1 report" --body-file report.md

# Replace a page body; --allow-merge resolves a concurrent edit instead of failing
cat report.md | ywiki page edit users/me/report --body-file - --allow-merge
```

`--body-file -` reads stdin, which is the right way to pass multi-line content.
Pass `--silent` on create, edit, or append to avoid notifying page subscribers
for routine automated updates.

## Comments and attachments

```bash
ywiki comment list users/me/notes --json id,author,body
ywiki comment create users/me/notes --body "Reviewed"
ywiki comment create users/me/notes --body "Agreed" --reply-to 42

ywiki attachment list users/me/notes --json id,name
ywiki attachment upload users/me/notes report.pdf
ywiki attachment download users/me/notes 987 --out report.pdf
```

## Tables

Dynamic tables (grids) are addressed by table ID, not slug. Find them on a page,
then read the schema before writing:

```bash
ywiki grid list users/me/notes --json id,title
ywiki grid get <id> --json columns,revision --jq '{revision, columns: [.columns[] | {slug, type}]}'
ywiki grid get <id> --jq '.rows'          # rows as objects keyed by column slug, row ID in "_id"
ywiki grid get <id> --filter '[status] ~ open' --sort -created --cols name,status
ywiki grid get <id> --format csv > table.csv
```

Rows and columns are keyed by column slug, never by position. Row IDs come from
`_id` (or `ywiki grid get <id> --quiet`).

```bash
# Add rows: JSON array, one JSON object per line, or CSV with a slug header row
ywiki grid row add <id> --rows '[{"name": "bolt", "qty": 10}]'
ywiki grid row add <id> --rows-file rows.csv --quiet       # prints the new row IDs
ywiki grid row remove <id> 3 4
ywiki grid row move <id> 7 --position 0

# Set cells
ywiki grid cell set <id> 3 qty 12
ywiki grid cell set <id> 3 note --clear
echo '[{"row_id": 3, "column": "qty", "value": 5}]' | ywiki grid cell set <id> --cells-file -

# Create a table, and show it on the page
ywiki grid create users/me/notes --title Tasks --column name:string --column done:checkbox --embed
ywiki grid column add <id> --column price:number:Price
```

- Input is checked against the schema first: an unknown column is rejected with
  the valid slugs listed, and CSV or `cell set` values are converted to the
  column type (numbers, `true`/`false`, select options as a list). Staff and
  ticket cells take JSON, e.g. `[{"uid": "123"}]`.
- Every write prints the new `revision`. The API does not reject a write made
  against an outdated revision, so concurrent edits to the same table are not
  detected: re-read the table before changing a cell someone else may have
  edited. `ywiki grid get <id> --revision <rev>` reads an older version.
- `--filter` uses the API's own syntax; it works on text and number columns,
  but select columns do not filter reliably.
- A table created with `grid create` is invisible until the page references it.
  Use `--embed`, or append `{% wgrid id="<id>" %}` to the page.
- `ywiki grid delete` deletes at once, like `page delete`: there is no
  confirmation step, so check the table ID first.

## Rules

- Deleting is permanent and does not ask for confirmation. Confirm with the
  user before deleting a page or table, and never delete one you did not create
  in this task.
- Page content is Yandex Wiki markup, close to Markdown but not identical.
  Read an existing page first when matching a house style.
- API requests act as the signed-in user, so a 403 means that person lacks
  access, not that the token is wrong. Report it rather than retrying.
- `ywiki page clone` runs asynchronously and waits by default; pass `--no-wait`
  only if you do not need the resulting slug.
