# nanoharness

A code-aware terminal harness for Codex, Prime Intellect (`prime-agent`), OpenAI, Anthropic, and pi.

`nanoharness` is written in Go and built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lip Gloss](https://github.com/charmbracelet/lipgloss). It takes interaction inspiration from Charm's Crush: a focused keyboard-first conversation, a compact inspector, clear model controls, and explicit permission gates. It is an independent implementation and does not copy Crush code.

## Layout

```text
cmd/nanoharness/          thin CLI entry (go install target)
internal/harness/         Superpower choke point (gather → send)
  types.go gather.go send.go format.go catalog.go
internal/tui/             Charm Bubbles UI (calls harness only)
internal/cli/             run / context / login (calls harness only)
internal/context/         local lexical retrieval
internal/providers/       Codex / OpenAI / Anthropic / pi transport
```

## Architecture (Prime Agent–inspired)

nanoharness adapts ideas from [Prime Agent](https://github.com/PrimeIntellect-ai/prime-agent)
(RLM + Continual Harness) without forking its runtime:

```text
prompt → Session.Gather (lexical cites + continual goal/memories)
      → confirm gate
      → Session.Send → provider (prime-agent / codex / openai / …)
```

- **Continual state:** `/goal`, `/memory`, `/auto`, `/gate` (TUI) or
  `--goal --auto --gate --max-turns` (CLI).
- **prime-agent:** print mode with optional `--goal` / `--autonomous` /
  `--autonomous-gate`; read-only uses `--no-tools` until write/auto is armed.
- **Host boundary:** only `harness.Send` talks to providers; TUI/CLI use Session.
- **Ghostty:** detects `TERM_PROGRAM=ghostty`, enables focus reporting, and shows
  terminal info in `/status` and the inspector (see `contrib/ghostty.conf`).

## Superpower

Every ask runs through one harness path (`internal/harness`):

1. Extract search terms from the prompt (stopwords stripped).
2. Gather bounded local lexical citations from the working tree.
3. Frame the provider request with a Superpower preamble + cited snippets.
4. Gate anything that leaves the machine behind an explicit confirmation.

TUI send, `nanoharness run`, `/query`/`/research`/`/impact`, and
`nanoharness context …` all go through a `harness.Session`
(`internal/tui` and `internal/cli` call Session methods only). Provider
HTTP/CLI calls are only made from `harness.Send`.

Superpower is **on by default** in the TUI and for `nanoharness run`
(`F5` / `/super off` / `--no-super` to disable).

Nothing is a semantic/repo-trained model, embedding index, vector database, or
dependency graph. Citations are incomplete evidence, not conclusions.

## Features

- Charm Bubbles TUI: viewport chat, list pickers, spinner, help, multi-line
  textarea composer, Superpower phase chips, Ghostty-aware focus reporting,
  and mouse wheel scrolling.
- Providers: official Codex CLI, **Prime Intellect prime-agent**, OpenAI
  Responses API, Anthropic Messages API, and local pi CLI.
- Provider-owned authentication. Codex browser login remains in the official
  CLI. API credentials are entered without echo and stored in an owner-only
  XDG config file; environment variables take priority.
- Codex read-only by default. Workspace writes require explicit arming and a
  send confirmation.
- Local cited code context: deterministic, bounded lexical search with exact
  file/line snippets (`query` / soft `research` / symbol-focused `impact`).
- A citation attachment gate: local source only leaves the machine after
  Superpower (or `/context on`) and an explicit confirmation that names the
  provider and citation count.

## Install

```sh
go install github.com/shubhxho/nanoharness/cmd/nanoharness@latest
# or pin: go install github.com/shubhxho/nanoharness/cmd/nanoharness@v0.11.0
nanoharness
nanoharness version
nanoharness status
```

Prime Agent (optional):

```sh
curl -fsSL https://app.primeintellect.ai/prime-agent/install.sh | sh
nanoharness login prime
nanoharness run --provider prime --goal "fix auth" --auto --gate "go test ./..." "implement the fix"
```

Local builds embed `git describe` + `git rev-parse --short HEAD`:

```sh
make version   # prints describe + rev-parse
make build
./bin/nanoharness version
```

That installs the `nanoharness` binary into `$(go env GOPATH)/bin` (ensure it is
on your `PATH`). TUI, `run`, and `context` all execute through the Superpower
harness after install.

For local development:

```sh
make install   # go install ./cmd/nanoharness
make test vet build
go run ./cmd/nanoharness
```
## TUI keys

| Key | Action |
| --- | --- |
| `Enter` | Send through harness (gather → confirm → send) |
| `Ctrl+J` | Insert newline in the composer |
| `↑` / `↓` | Prompt history (when composer empty / browsing) |
| `F1` / `?` | Toggle Bubbles help |
| `F2` / `Ctrl+P` | Provider list (filterable) |
| `F3` | Model list (filterable) |
| `F4` | Toggle attaching preloaded citations |
| `F5` | Toggle Superpower |
| `F6` | Session status |
| `Tab` | Next provider |
| `Ctrl+N` | New session (clear chat + continual state) |
| `Ctrl+W` | Arm/disarm Codex workspace write |
| `y` / `Enter` | Approve a pending Superpower send |
| `n` / `Esc` | Cancel a pending send |
| `PgUp` / `PgDn` | Scroll chat viewport |
| Mouse wheel | Scroll chat |
| `Ctrl+C` | Quit |

Commands in the composer:

```text
/super on
/super off
/goal ship the auth fix
/memory prefer session tokens over cookies
/auto on
/gate go test ./...
/gates
/memories
/status
/terminal
/query rate limiting inbound webhook
/research where auth is checked
/impact requireUser
/context on
/context off
/context clear
/provider prime
/model gpt-5.6-terra
/new
```

## Ghostty

When `TERM_PROGRAM=ghostty` (or `TERM=xterm-ghostty`), nanoharness enables
focus reporting, shows a **GHOSTTY** chip in the header, and prints terminal
details in `/status` and the inspector sidebar.

Sample keybind (merge into `~/.config/ghostty/config`):

```ini
keybind = super+shift+n=text:nanoharness\r
```

See [contrib/ghostty.conf](contrib/ghostty.conf) for a fuller starter snippet.

## Local context contract

The context engine is local **lexical** search. It matches exact lower-cased
path and content tokens, returns ranked citations, and is deliberately not a
semantic model.

| Mode | Behavior |
| --- | --- |
| `query` | Every token must hit path or content |
| `research` | Soft OR — keep files matching enough tokens |
| `impact` | Prefer exact identifier / symbol hits |

It reads only regular UTF-8 files under the working root. It skips symlinks,
dot-directories, `.git`, `target`, `node_modules`, `.venv`, `vendor`, common
binary file types, and files over 1 MiB. Searches are capped at 8 MiB scanned,
25 citations, 80 lines per citation, and 32 KiB attached context. Context
snippets are transient and are not persisted in session state.

CLI commands:

```sh
nanoharness context index
nanoharness context query "rate limiting inbound webhook"
nanoharness context research "where is auth enforced"
nanoharness context impact "requireUser"
```

## Login and scripting

```sh
nanoharness login codex
nanoharness login codex --api-key
nanoharness login openai
nanoharness login anthropic
nanoharness login prime

nanoharness status
nanoharness run --provider prime "review this project"
nanoharness run --provider codex "review this project"
nanoharness run --provider openai --model gpt-5.6-terra "where is rate limiting?"
nanoharness run --no-super --provider anthropic --model claude-sonnet-5 "say hi"
nanoharness run --provider pi --model openai-codex/gpt-5.6-terra "say hi"
```

A Codex 401 means the official Codex session needs a new `nanoharness login
codex`. Never copy or edit vendor credential files.

## Build and release

GitHub Actions runs formatting, `go vet`, race-enabled tests, and native builds
on Linux, macOS Intel, macOS Apple Silicon, and Windows. Pushing a `v*` tag
runs GoReleaser to publish platform archives, checksums, and SBOMs as a GitHub
Release.

```sh
make fmt test vet build
# Maintainers: tag a validated version, then push the tag.
git tag v0.11.0
git push origin v0.11.0
```

## License

MIT. See [LICENSE](LICENSE).
