---
name: ywiki
description: Read and write Yandex Wiki pages, search the wiki, and manage comments and attachments from the command line. Use when a task involves Yandex Wiki content - reading a page, publishing notes or a report to the wiki, searching internal documentation, or commenting on a page.
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
  5 rate limited.

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

## Rules

- Deleting is destructive and needs `--yes` when not attended. Confirm with the
  user before deleting a page, and never delete one you did not create in this
  task.
- Page content is Yandex Wiki markup, close to Markdown but not identical.
  Read an existing page first when matching a house style.
- API requests act as the signed-in user, so a 403 means that person lacks
  access, not that the token is wrong. Report it rather than retrying.
- `ywiki page clone` runs asynchronously and waits by default; pass `--no-wait`
  only if you do not need the resulting slug.
