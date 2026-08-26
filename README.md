# nanoharness

A code-aware terminal harness for Codex, OpenAI, Anthropic, and pi.

`nanoharness` is written in Go and built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lip Gloss](https://github.com/charmbracelet/lipgloss). It takes interaction inspiration from Charm's Crush: a focused keyboard-first conversation, a compact inspector, clear model controls, and explicit permission gates. It is an independent implementation and does not copy Crush code.

## Features

- Charm-style full-screen TUI with a styled status bar, rounded composer,
  provider/model pickers, responsive inspector, and local-context state.
- Providers: official Codex CLI, OpenAI Responses API, Anthropic Messages API,
  and local pi CLI.
- Provider-owned authentication. Codex browser login remains in the official
  CLI. API credentials are entered without echo and stored in an owner-only
  XDG config file; environment variables take priority.
- Codex read-only by default. Workspace writes require explicit arming and a
  send confirmation.
- Local cited code context: deterministic, bounded local lexical search with
  exact file/line snippets.
- A citation attachment gate: local source only leaves the machine after
  `/context on` and an explicit confirmation that names the provider and
  citation count.

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
| `Enter` | Send the composer text |
| `F1` | Help and command palette |
| `F2` / `Ctrl+P` | Provider picker |
| `F3` | Model picker |
| `F4` | Toggle selected local citations for the next provider request |
| `Tab` | Next provider |
| `Ctrl+W` | Arm/disarm Codex workspace write |
| `Ctrl+C` | Quit |

Commands in the composer:

```text
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
semantic/repo-trained model, embedding index, vector database, or dependency
graph. Results are incomplete evidence, not conclusions.

It reads only regular UTF-8 files under the working root. It skips symlinks,
`.git`, `target`, `node_modules`, `.venv`, `vendor`, common binary file types,
and files over 1 MiB. Searches are capped at 8 MiB scanned, 25 citations, 80
lines per citation, and 32 KiB attached context. Context snippets are transient
and are not persisted in session state.

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
nanoharness run --provider openai --model gpt-5.6-terra "say hi"
nanoharness run --provider anthropic --model claude-sonnet-5 "say hi"
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
git tag v0.2.0
git push origin v0.2.0
```

## License

MIT. See [LICENSE](LICENSE).
