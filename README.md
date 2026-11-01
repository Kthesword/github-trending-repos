# trending-repos

[![CI](https://github.com/Kthesword/github-trending-repos/actions/workflows/ci.yml/badge.svg)](https://github.com/Kthesword/github-trending-repos/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Kthesword/github-trending-repos)](https://github.com/Kthesword/github-trending-repos/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Repository:** <https://github.com/Kthesword/github-trending-repos>

A fast, dependency-light command-line tool, written in Go, that lists **trending public GitHub repositories** in your terminal. It uses the [GitHub REST Search API](https://docs.github.com/en/rest/search/search#search-repositories) and ranks results by star count.

```
  Trending on GitHub  · past 7 days (since Oct 25, 2026 10:02)

  #   REPOSITORY                    STARS   LANGUAGE     DESCRIPTION
  ──────────────────────────────────────────────────────────────────────────────────
  1   acme/rocket-db               12,345   Rust         An embeddable, blazing-fast key-value store
  2   someone/agent-kit             8,902   TypeScript   Toolkit for building autonomous agents
  3   example/tiny-shell            4,117   Go           A minimal POSIX shell in under 2k lines
  4   octo/notes                    2,010   -            -

  Showing 4 of 48,213 matching repositories, ranked by stars.
```

## Features

- **Configurable time range**: `day`, `week`, `month`, or `year`.
- **Configurable result count**: 1 to 1,000. Paginates automatically past 100.
- **Sorted by stars**, most-starred first. Ties are broken by name so the order is stable.
- **Aligned, colored table output**. Colors turn off automatically when output is piped, and you can disable them with `--no-color` or `NO_COLOR`.
- **Clear error messages** for invalid input, network failures, timeouts, rate limits (HTTP 403/429), and other non-200 responses.
- **Safe output**: control characters and ANSI escape sequences in repository descriptions are stripped, so a malicious description can't change your terminal.
- **Ctrl+C cancels** in-flight requests cleanly.
- **No authentication needed**. You can set an optional `GITHUB_TOKEN` to get higher rate limits.

## Requirements

- [Go](https://go.dev/dl/) **1.25** or newer (only needed to build from source)
- Internet access to `api.github.com`

## Installation

### Option 1: `go install`

```bash
go install github.com/kthesword/github-trending-repos@latest
```

This puts a `github-trending-repos` binary in `$(go env GOPATH)/bin`. Make sure that directory is on your `PATH`. To get the shorter `trending-repos` name, use Option 2 or download a release binary (Option 3).

### Option 2: Build from source

```bash
git clone https://github.com/Kthesword/github-trending-repos.git
cd github-trending-repos
go mod download
```

**Linux / macOS**

```bash
go build -o trending-repos .
./trending-repos --help
```

**Windows (PowerShell)**

```powershell
go build -o trending-repos.exe .
.\trending-repos.exe --help
```

### Build with an embedded version

```bash
go build -ldflags "-s -w -X github.com/kthesword/github-trending-repos/cmd.version=v1.0.0" -o trending-repos .
./trending-repos --version
```

### Cross-compile

```bash
# Linux / macOS shells
GOOS=linux   GOARCH=amd64 go build -o dist/trending-repos-linux-amd64 .
GOOS=darwin  GOARCH=arm64 go build -o dist/trending-repos-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -o dist/trending-repos-windows-amd64.exe .
```

```powershell
# PowerShell
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o dist/trending-repos-linux-amd64 .
Remove-Item Env:GOOS, Env:GOARCH
```

## Usage

```
trending-repos [flags]
```

### Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--duration` | `-d` | string | `week` | Time range: `day`, `week`, `month`, `year` (case-insensitive) |
| `--limit` | `-l` | int | `10` | Maximum number of repositories to show (1-1000) |
| `--timeout` | | duration | `15s` | Timeout for each HTTP request (for example `30s` or `1m`) |
| `--no-color` | | bool | `false` | Turn off colored output |
| `--version` | `-v` | | | Print the version |
| `--help` | `-h` | | | Show help |

### Examples

```bash
# Top 10 repositories created in the past week (defaults)
trending-repos

# Top 20 repositories created in the past month
trending-repos --duration month --limit 20

# Today's top 5, using the short flags
trending-repos -d day -l 5

# The year's 250 most-starred new repositories (fetched over 3 pages)
trending-repos --duration year --limit 250

# Plain output for logs or piping
trending-repos --no-color > trending.txt

# Use a token for higher rate limits
export GITHUB_TOKEN=ghp_yourTokenHere          # bash / zsh
$env:GITHUB_TOKEN = "ghp_yourTokenHere"        # PowerShell
trending-repos --duration month --limit 100
```

### Environment variables

| Variable | Purpose |
|----------|---------|
| `GITHUB_TOKEN` | Optional [personal access token](https://github.com/settings/tokens). It needs no scopes, because only public data is read. It raises the search limit from 10 to 30 requests per minute. |
| `GITHUB_API_URL` | Optional API base URL, for example `https://github.example.com/api/v3` for GitHub Enterprise Server. Defaults to `https://api.github.com`. |
| `NO_COLOR` | If set, colors are turned off ([no-color.org](https://no-color.org)). |
| `HTTPS_PROXY` / `NO_PROXY` | Standard proxy settings, which are respected automatically. |

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Runtime failure: network error, rate limit, or GitHub API error |
| `2` | Usage error: invalid flag value or unexpected argument |
| `130` | Interrupted with Ctrl+C |

## How "trending" is defined

GitHub's public API has no "trending" endpoint. This tool uses the common approximation: repositories **created** inside the selected window, ranked by **stars**. For example, `--duration week` sends this request:

```
GET /search/repositories?q=created:>=2026-10-25T07:02:43Z is:public&sort=stars&order=desc&per_page=10&page=1
```

- The start of the window comes from `time.Now().AddDate(...)`: −1 day, −7 days, −1 month, or −1 year. It is sent as a UTC timestamp accurate to the second, so even `day` is precise.
- `is:public` makes sure that only public repositories appear, even when a token with private access is set.
- The API already sorts by stars. The CLI sorts the results again locally (descending, with name as the tie-breaker) so the order stays correct across pages.

## Error handling

Errors are short and include a next step:

```text
$ trending-repos --duration decade
Error: invalid duration "decade": must be one of day, week, month, year
  → Run 'trending-repos --help' for usage.

$ trending-repos --limit 5000
Error: invalid limit 5000: must be between 1 and 1000
  → Run 'trending-repos --help' for usage.

$ trending-repos            # after too many requests
Error: GitHub API rate limit exceeded (HTTP 403).
  → Try again in 42s (at 10:15:03).
  → Unauthenticated clients get 10 searches per minute; set GITHUB_TOKEN to a personal access token for a higher limit.

$ trending-repos            # while offline
Error: Could not reach GitHub: dial tcp: lookup api.github.com: no such host
  → Check your internet connection, DNS, and proxy settings (HTTPS_PROXY).
```

The tool treats **HTTP 429**, and **HTTP 403** with `X-RateLimit-Remaining: 0`, a `Retry-After` header, or a "rate limit" message, as rate limiting. It reads the reset time from `X-RateLimit-Reset` or `Retry-After`. Other non-200 responses (401, 422, 5xx, and so on) get their own messages, including any detail GitHub sends.

## Project structure

```
.
├── main.go                 # Entry point: os.Exit(cmd.Execute())
├── cmd/                    # CLI layer (Cobra)
│   ├── root.go             #   flags, validation, wiring, signal handling
│   ├── errors.go           #   error → friendly message, hints, exit code
│   └── root_test.go
├── api/                    # GitHub HTTP client
│   ├── client.go           #   Client, functional options, headers, JSON decoding
│   ├── search.go           #   query building, pagination, de-duplication
│   ├── errors.go           #   RateLimitError, StatusError, NetworkError
│   └── client_test.go      #   httptest-based tests
├── models/                 # Data types and domain logic
│   ├── repository.go       #   API JSON structs, SortByStars
│   ├── period.go           #   --duration parsing and date calculation
│   └── *_test.go
├── formatter/              # Terminal rendering
│   ├── table.go            #   aligned, color-aware table
│   └── table_test.go
├── go.mod
└── README.md
```

**Design notes**

- **One-way dependencies:** `cmd` → `api`, `formatter` → `models`. The `api` package doesn't depend on the terminal, and the `formatter` package doesn't depend on HTTP, so each can be reused or tested alone.
- **Typed errors** in `api` let `cmd` choose a message with `errors.As` instead of matching strings.
- **Alignment with colors:** `text/tabwriter` counts ANSI escape bytes as visible width, which breaks alignment once colors are added. The formatter pads the plain text first and then adds color.
- **Testable root command:** `newRootCmd()` builds a fresh command each time, so tests don't share global Cobra state.

## Deployment

### Publish a release (prebuilt binaries)

Pushing a version tag triggers [`release.yml`](.github/workflows/release.yml), which runs [GoReleaser](https://goreleaser.com) and publishes a GitHub Release with archives for Linux, macOS and Windows (amd64 and arm64) plus a `checksums.txt`.

```bash
git tag -a v1.0.0 -m "v1.0.0"
git push origin v1.0.0
```

No secrets are needed; the workflow uses the built-in `GITHUB_TOKEN`. To try a release locally without publishing: `goreleaser release --snapshot --clean`.

### Option 3: Download a release binary

Grab the archive for your platform from the [Releases page](https://github.com/Kthesword/github-trending-repos/releases), extract it, and put `trending-repos` on your `PATH`.

### Docker

```bash
docker build -t trending-repos .
docker run --rm trending-repos --duration month --limit 20

# with a token for higher rate limits
docker run --rm -e GITHUB_TOKEN trending-repos -d day
```

The image is a static binary on `distroless`, runs as a non-root user, and has no shell.

### Continuous integration

[`ci.yml`](.github/workflows/ci.yml) runs on every push and pull request: gofmt, `go mod tidy` and `go vet` checks, tests on Linux, macOS and Windows (with the race detector on Linux), and a GoReleaser dry run.

## Development

```bash
go test ./...                 # run all tests (no network access needed)
go test -cover ./...          # with coverage
go vet ./...                  # static analysis
gofmt -l .                    # list unformatted files
```

The tests use `net/http/httptest` to simulate GitHub, covering success, pagination, 403/429 rate limits, 422/5xx errors, malformed JSON, timeouts, and cancellation. They never call the real API.

## Rate limits at a glance

| Mode | Search requests per minute |
|------|----------------------------|
| Unauthenticated | 10 |
| With `GITHUB_TOKEN` | 30 |

Each run uses one request per 100 results. For example, `--limit 250` uses 3 requests.

## License

MIT
