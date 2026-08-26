# nanoharness

A small Rust CLI for using Codex or Anthropic without a framework.

- `codex` uses the official Codex CLI. Its login stays in `~/.codex`.
- `openai` calls the OpenAI Responses API with an API key.
- `anthropic` calls the Anthropic Messages API with an API key.
- Keys are stored in `~/.config/nanoharness/credentials` with owner-only permissions on Unix. Environment variables override this file.

## Install

```sh
cargo install --path .
```

## Login

```sh
nanoharness login codex             # official browser login
nanoharness login codex --api-key   # hidden API-key prompt, then official Codex login
nanoharness login openai            # hidden OPENAI_API_KEY prompt
nanoharness login anthropic         # hidden ANTHROPIC_API_KEY prompt
nanoharness login claude            # optional: native Claude Code browser login
```

The Anthropic API does not provide a general browser OAuth flow for third-party
clients. `login anthropic` therefore stores an API key. `login claude` delegates
to Claude Code's native browser flow and never reads its credentials.

## Run

```sh
nanoharness run "explain this repository"
nanoharness run --provider codex "review the code"
nanoharness run --provider openai --model gpt-5-mini "say hi"
nanoharness run --provider anthropic --model claude-sonnet-4-5 "say hi"
```

`run` chooses OpenAI API, then Codex login, then Anthropic API when no provider
is specified. Codex runs read-only by default. Pass `--write` only when you want
the Codex agent to edit the working directory.

## Why this exists

It borrows only small-tool ideas from pi, Superpowers, and Prime Agent: a tiny
command surface, explicit write permission, provider-owned authentication, and
no local agent framework. It does not copy their code or credentials.
