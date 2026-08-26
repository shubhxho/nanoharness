# nanoharness

A compact Rust terminal harness for Codex, OpenAI, Anthropic, and pi.

## Features

- Native alternate-screen TUI. `nanoharness` starts it directly.
- Four backends: official Codex CLI, OpenAI Responses API, Anthropic Messages
  API, and the local pi CLI.
- `p` provider picker with non-secret readiness state. It reports native CLI
  login, configured API key, or a missing dependency. It never validates or
  displays credentials in the UI.
- `m` model picker with suggested current models, plus `/model NAME` for any
  account-specific model. Codex and pi default to their own selected default.
- Non-secret session history in `~/.local/state/nanoharness/session.json` (or
  `$XDG_STATE_HOME`). It keeps the last 200 transcript entries. API keys are
  never written there.
- Scrollable transcript, full error detail (`e`), provider errors kept in the
  conversation, and responsive background provider work.
- Codex is read-only by default. A workspace-write request requires arming it
  and then confirming that specific prompt. The mode turns off after each write
  request and is never persisted.
- One-shot CLI for scripts and automation.

## Install and start

```sh
cargo install --path .
nanoharness
```

During development, Cargo needs `--` before program arguments:

```sh
cargo run                         # open the TUI
cargo run -- login codex          # run the official Codex browser login
cargo run -- run --provider codex "review this project"
```

## TUI controls

| Key or command | Action |
| --- | --- |
| `Enter` | Send the prompt |
| `p`, `m` | Open provider or model picker; `↑`/`↓` select, `Enter` confirms |
| `Tab` | Cycle backends quickly |
| `↑`/`↓`, `Home`/`End` | Scroll transcript or return to the newest message |
| `Ctrl+W` or `/write` | Arm or disarm Codex workspace writing |
| `/provider NAME`, `/model NAME` | Select a provider or any custom model |
| `/status` | Refresh the selected backend's readiness state |
| `/new` or `/clear` | Start a new transcript |
| `e` | Open the latest provider error detail |
| `/help`, `/exit`, `q`, `Ctrl+C` | Get help or leave the TUI |

Suggested API defaults are `gpt-5.6-terra` for OpenAI and `claude-sonnet-5`
for Anthropic. These are account-dependent; use the picker or `/model` to
change them. A bare `gpt5` is not a valid model identifier.

## Login

```sh
nanoharness login codex             # official browser login
nanoharness login codex --api-key   # hidden API-key prompt to official Codex login
nanoharness login openai            # hidden OPENAI_API_KEY prompt
nanoharness login anthropic         # hidden ANTHROPIC_API_KEY prompt
nanoharness login claude            # optional native Claude Code browser login
```

The Anthropic API has no general third-party browser OAuth flow. `login
anthropic` stores an API key. `login claude` delegates to Claude Code and never
reads its credentials. A Codex 401 means the official Codex session needs a
new login:

```sh
nanoharness login codex
```

Do not copy or edit vendor credential files.

## One-shot CLI

```sh
nanoharness run --provider codex "review the code"
nanoharness run --provider openai --model gpt-5.6-terra "say hi"
nanoharness run --provider anthropic --model claude-sonnet-5 "say hi"
nanoharness run --provider pi --model openai-codex/gpt-5.6-terra "say hi"
```

## Influences

The interface borrows small-tool ideas from pi, Superpowers, and Prime Agent:
a tight interactive surface, model/provider controls, explicit write
permission, and provider-owned authentication. It does not copy their source
code or credentials.
