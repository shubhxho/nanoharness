# nanoharness

A compact Rust terminal harness for Codex, OpenAI, Anthropic, pi, and cited
local code context.

## What it does

- A polished, Charm-inspired native terminal UI with a dark surface palette,
  provider and model chips, a responsive inspector, rounded composer, command
  palette, picker overlays, and real input cursor/paste support.
- Four execution backends: the official Codex CLI, OpenAI Responses API,
  Anthropic Messages API, and the local pi CLI.
- Provider/model controls, API-key/CLI readiness indicators, read-only Codex by
  default, per-request write confirmation, and transcript/error history.
- **Perseus-style local context:** bounded filesystem search with file and line
  citations. `query`, `research`, and `impact` find local exact token/path
  matches before a provider request.

## Local context contract

`nanoharness context` is a deterministic local lexical search tool. It does
**not** use embeddings, a vector database, a trained repo model, or a semantic
dependency graph. Results are incomplete evidence, not answers or proven call
edges.

The engine only reads regular UTF-8 files under the selected root. It skips
symlinks, `.git`, `target`, `node_modules`, `.venv`, common binary extensions,
and files over 1 MiB. A query scans at most 8 MiB, returns at most 25 results,
and emits exact 1-based file/line citations.

Source excerpts remain local by default. In the TUI, use `/query` first, then
`/context on` to attach the selected excerpts to a provider request. Before any
excerpt leaves the machine, nanoharness shows a confirmation dialog with the
provider and citation count. Excerpts are never written to session history.

## Install and start

```sh
cargo install --path .
nanoharness
```

During development, Cargo needs `--` before application arguments:

```sh
cargo run
cargo run -- login codex
cargo run -- context query "auth boundary"
cargo run -- run --provider codex "review this project"
```

## TUI controls

| Key or command | Action |
| --- | --- |
| `Enter` | Send the composer text |
| `F2` or `Ctrl+P` | Provider picker |
| `F3` | Model picker |
| `F4` | Toggle local-context attachment |
| `Tab` | Next provider |
| `←` `→` `Home` `End` | Move the input cursor |
| `PageUp` `PageDown` | Browse transcript history |
| `Ctrl+W` or `/write` | Arm Codex workspace writing |
| `Ctrl+E` | Open the latest error detail |
| `F1` or `/help` | Command palette/help |
| `/query TERMS` | Find local cited code excerpts |
| `/research QUESTION` | Create a local cited evidence packet |
| `/impact SYMBOL` | Find possible lexical references, not a dependency graph |
| `/context on\|off\|clear\|status` | Manage transient local citations |
| `/new`, `/status`, `/exit`, `Ctrl+C` | Start over, refresh auth, or leave |

Suggested API defaults are `gpt-5.6-terra` for OpenAI and `claude-sonnet-5`
for Anthropic. Availability is account-specific; use F3 or `/model NAME` to
change them. Do not use a bare `gpt5` model name.

## Context CLI

```sh
nanoharness context index
nanoharness context query -n 8 "rate limiting inbound webhook"
nanoharness context research "where auth is checked for query routes"
nanoharness context impact "requireUser"
nanoharness context query --root ../another-repo "session helper"
```

Each command prints a clear local-lexical disclaimer, ranked citations, scores,
and numbered source lines. `research` changes only the evidence label;
`impact` is explicitly a possible lexical impact list.

## Login

```sh
nanoharness login codex             # official browser login
nanoharness login codex --api-key   # hidden API-key prompt to official Codex login
nanoharness login openai            # hidden OPENAI_API_KEY prompt
nanoharness login anthropic         # hidden ANTHROPIC_API_KEY prompt
nanoharness login claude            # optional native Claude Code browser login
```

A Codex 401 means the official Codex session needs a new login:

```sh
nanoharness login codex
```

Never copy or edit vendor credential files.

## One-shot CLI

```sh
nanoharness run --provider codex "review the code"
nanoharness run --provider openai --model gpt-5.6-terra "say hi"
nanoharness run --provider anthropic --model claude-sonnet-5 "say hi"
nanoharness run --provider pi --model openai-codex/gpt-5.6-terra "say hi"
```

## Influences

The interface borrows small-tool ideas from Charm, pi, Superpowers, Prime
Agent, and cited-code-context workflows: a focused terminal surface, explicit
permissions, local evidence before edits, and provider-owned authentication.
It does not copy their source code or credentials.
