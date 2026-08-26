# nanoharness

A small Rust terminal harness for Codex, OpenAI, Anthropic, and pi.

- The default command opens a native terminal UI (TUI).
- `codex` delegates login and model access to the official Codex CLI.
- `pi` delegates to the local pi CLI, so pi keeps its own provider settings.
- `openai` and `anthropic` call their APIs with an API key.
- API keys are stored in `~/.config/nanoharness/credentials` with owner-only
  permissions on Unix. Environment variables override this file.

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
| `Tab` | Cycle `codex`, `openai`, `anthropic`, and `pi` |
| `Ctrl+W` or `/write` | Toggle Codex workspace-write mode |
| `/provider NAME` | Select a backend |
| `/model NAME` | Set the model; leave unset for the vendor default |
| `/clear`, `/help`, `/exit` | Clear, show help, or exit |
| `q` on an empty prompt or `Ctrl+C` | Exit |

The TUI runs one request at a time and shows provider errors in the transcript.
Codex is read-only by default. Workspace writing requires an explicit toggle.

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
reads its credentials. If Codex shows a 401, run `nanoharness login codex`; do
not copy or edit `~/.codex` credentials.

## One-shot CLI

```sh
nanoharness run --provider codex "review the code"
nanoharness run --provider openai --model gpt-5-mini "say hi"
nanoharness run --provider anthropic --model claude-sonnet-4-5 "say hi"
nanoharness run --provider pi --model openai/gpt-5.5 "say hi"
```

## Why this exists

It borrows small-tool ideas from pi, Superpowers, and Prime Agent: a tight
interactive surface, explicit write permission, provider-owned authentication,
and no copied credentials or agent framework.
