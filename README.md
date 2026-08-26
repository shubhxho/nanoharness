# nanoharness

A code-aware terminal harness for Codex, OpenAI, Anthropic, and pi.

`nanoharness` is written in Go and built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lip Gloss](https://github.com/charmbracelet/lipgloss). It takes interaction inspiration from Charm's Crush: a focused keyboard-first conversation, a compact inspector, clear model controls, and explicit permission gates. It is an independent implementation and does not copy Crush code.

## Superpower

Every ask runs through one harness path (`internal/harness`):

1. Extract search terms from the prompt (stopwords stripped).
2. Gather bounded local lexical citations from the working tree.
3. Frame the provider request with a Superpower preamble + cited snippets.
4. Gate anything that leaves the machine behind an explicit confirmation.

TUI send, `nanoharness run`, `/query`/`/research`/`/impact`, and
`nanoharness context …` all go through that package. Provider HTTP/CLI calls
are only made from `harness.Send`.

Superpower is **on by default** in the TUI and for `nanoharness run`
(`F5` / `/super off` / `--no-super` to disable).

Nothing is a semantic/repo-trained model, embedding index, vector database, or
dependency graph. Citations are incomplete evidence, not conclusions.

## Features

- Charm-style full-screen TUI with a styled status bar, Superpower chip, rounded
  composer, provider/model pickers, responsive inspector, and local-context state.
- Providers: official Codex CLI, OpenAI Responses API, Anthropic Messages API,
  and local pi CLI.
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
nanoharness
```

For development:

```sh
go run ./cmd/nanoharness
go test -race ./...
go vet ./...
```

## TUI keys

| Key | Action |
| --- | --- |
| `Enter` | Send the composer text through the harness |
| `F1` | Help and command palette |
| `F2` / `Ctrl+P` | Provider picker |
| `F3` | Model picker |
| `F4` | Toggle attaching preloaded citations (when Super is off) |
| `F5` | Toggle Superpower |
| `Tab` | Next provider |
| `Ctrl+W` | Arm/disarm Codex workspace write |
| `y` / `Enter` | Approve a pending Superpower send |
| `n` / `Esc` | Cancel a pending send |
| `PgUp` / `Ctrl+U` | Scroll chat up |
| `PgDn` / `Ctrl+D` | Scroll chat down |
| `Ctrl+C` | Quit |

Commands in the composer:

```text
/super on
/super off
/query rate limiting inbound webhook
/research where auth is checked
/impact requireUser
/context on
/context off
/context clear
/provider anthropic
/model claude-sonnet-5
/new
```

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
git tag v0.3.0
git push origin v0.3.0
```

## License

MIT. See [LICENSE](LICENSE).
