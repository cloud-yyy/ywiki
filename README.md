# ywiki

Yandex Wiki CLI for humans and LLM agents.

A command-line client for [Yandex Wiki](https://wiki.yandex.ru/). Designed for
LLM agents calling commands programmatically, with a strong secondary focus on
developers working in a terminal.

## Features

- Page management: read, list, create, edit, append, copy, delete
- Full-text search across the wiki
- Comments and threads
- Attachments: list, upload, download, delete
- Slug or numeric page ID accepted everywhere
- Structured output with `--json` field selection and `--jq` filtering
- Shell completions for bash, zsh, and fish
- Semantic exit codes and predictable, parseable output

## Installation

### Homebrew

```
brew install cloud-yyy/tap/ywiki
```

### Go install

```
go install github.com/cloud-yyy/ywiki/cmd/ywiki@latest
```

### Binary download

Download a release from [GitHub Releases](https://github.com/cloud-yyy/ywiki/releases).

## Authentication

Yandex Wiki API requests act on behalf of a person: service accounts are not
supported, and every command is limited by that user's own Wiki permissions.

Three organization and token pairings are supported:

| Organization | `--org-type` | `--token-type` | Token |
| --- | --- | --- | --- |
| Yandex 360 for Business | `360` | `oauth` | OAuth token |
| Yandex Identity Hub | `cloud` | `oauth` | OAuth token |
| Yandex Cloud | `cloud` | `iam` | IAM token, valid for 12 hours |

Federated accounts must use an IAM token.

To get an OAuth token, create an app at [oauth.yandex.ru](https://oauth.yandex.ru)
("For API access or debugging") with the `wiki:write` or `wiki:read` permission,
then open `https://oauth.yandex.ru/authorize?response_type=token&client_id=<ClientID>`
while signed in as the account ywiki should act as.

The organization ID is listed under Administration, Organizations in
[Yandex Tracker](https://tracker.yandex.ru/admin/orgs).

Log in once:

```
ywiki auth login --org-id <your-org-id>
```

The token is read from a no-echo prompt, verified against the API, and stored
in `~/.config/ywiki/config.yaml` with `0600` permissions. Organization and token
type are detected when omitted. To feed the token from a secret store instead:

```
cat token.txt | ywiki auth login --org-id <your-org-id> --with-token
```

Credentials resolve in three tiers, highest first:

1. Flags: `--token`, `--org-id`, `--org-type` together, plus optional `--token-type`
2. Environment: `YWIKI_TOKEN`, `YWIKI_ORG_ID`, `YWIKI_ORG_TYPE`, plus optional `YWIKI_TOKEN_TYPE`
3. Config file written by `ywiki auth login`

Token type defaults to `oauth` for `360` and `iam` for `cloud`.

## Quick start

```bash
# Read a page
ywiki page get users/me/notes
ywiki page get users/me/notes --content > notes.md

# Create and update pages
ywiki page create users/me/notes --title Notes --body-file notes.md
ywiki page edit users/me/notes --title "Release notes"
git log -1 --oneline | ywiki page append users/me/log --body-file - --location top

# Browse and search
ywiki page list users/me --recursive
ywiki search "release checklist" --limit 50

# Comments
ywiki comment list users/me/notes
ywiki comment create users/me/notes --body "Reviewed"

# Attachments
ywiki attachment upload users/me/notes report.pdf
ywiki attachment download users/me/notes 987 --out report.pdf

# Machine-readable output
ywiki page list users/me --json id,slug
ywiki search onboarding --jq '.items[].slug'
ywiki page list users/me --quiet
```

## Output modes

| Mode | Flag | Use |
| --- | --- | --- |
| Table | default | Human reading in a terminal |
| JSON | `--json <fields>` | Selected fields as a JSON object or array |
| JSON + jq | `--jq <expr>` | Filter the full response, including pagination metadata |
| Quiet | `--quiet` | Primary identifiers only, one per line |

`--json` with no field names lists the available fields for that command.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Invalid input or usage |
| 3 | Authentication or permission failure |
| 4 | Page or resource not found |
| 5 | Rate limited |
| 130 | Interrupted |

## Documentation

- `ywiki --help` lists the commands
- `ywiki <command> --help` documents one command
- [SKILL.md](skills/ywiki/SKILL.md) is the integration guide for LLM agents

## License

MIT. See [LICENSE](LICENSE).
