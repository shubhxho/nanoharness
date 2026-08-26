use reqwest::blocking::Client;
use serde_json::{Value, json};
use std::{collections::BTreeMap, env, fs, path::PathBuf, process::Command};

pub const NAMES: [&str; 4] = ["codex", "openai", "anthropic", "pi"];

fn credentials_path() -> PathBuf {
    env::var_os("XDG_CONFIG_HOME")
        .map(PathBuf::from)
        .or_else(|| env::var_os("HOME").map(|h| PathBuf::from(h).join(".config")))
        .unwrap_or_else(|| PathBuf::from("."))
        .join("nanoharness/credentials")
}
fn credentials() -> BTreeMap<String, String> {
    fs::read_to_string(credentials_path())
        .unwrap_or_default()
        .lines()
        .filter_map(|line| {
            line.split_once('=')
                .map(|(k, v)| (k.to_owned(), v.to_owned()))
        })
        .collect()
}
fn key(name: &str) -> Option<String> {
    env::var(name).ok().or_else(|| credentials().remove(name))
}
fn text(v: &[Value]) -> Option<&str> {
    v.iter().find_map(|x| x.get("text").and_then(Value::as_str))
}
fn response(response: reqwest::blocking::Response) -> Result<Value, String> {
    let status = response.status();
    let body = response.text().unwrap_or_default();
    if status.is_success() {
        serde_json::from_str(&body).map_err(|e| e.to_string())
    } else {
        Err(format!("API returned {status}: {body}"))
    }
}
fn openai(prompt: &str, model: &str) -> Result<String, String> {
    let token =
        key("OPENAI_API_KEY").ok_or("missing OPENAI_API_KEY; run `nanoharness login openai`")?;
    let v = response(
        Client::new()
            .post("https://api.openai.com/v1/responses")
            .bearer_auth(token)
            .json(&json!({"model": model, "input": prompt}))
            .send()
            .map_err(|e| e.to_string())?,
    )?;
    v.get("output")
        .and_then(Value::as_array)
        .and_then(|out| {
            out.iter().find_map(|item| {
                item.get("content")
                    .and_then(Value::as_array)
                    .and_then(|content| text(content))
            })
        })
        .map(str::to_owned)
        .ok_or_else(|| format!("no text in response: {v}"))
}
fn anthropic(prompt: &str, model: &str) -> Result<String, String> {
    let token = key("ANTHROPIC_API_KEY")
        .ok_or("missing ANTHROPIC_API_KEY; run `nanoharness login anthropic`")?;
    let v = response(Client::new().post("https://api.anthropic.com/v1/messages").header("x-api-key", token).header("anthropic-version", "2023-06-01").json(&json!({"model": model, "max_tokens": 4096, "messages": [{"role": "user", "content": prompt}]})).send().map_err(|e| e.to_string())?)?;
    v.get("content")
        .and_then(Value::as_array)
        .and_then(|content| text(content))
        .map(str::to_owned)
        .ok_or_else(|| format!("no text in response: {v}"))
}
fn command(mut command: Command, name: &str) -> Result<String, String> {
    let output = command
        .output()
        .map_err(|_| format!("{name} CLI not found"))?;
    let stdout = String::from_utf8_lossy(&output.stdout).trim().to_owned();
    let stderr = String::from_utf8_lossy(&output.stderr).trim().to_owned();
    if output.status.success() {
        Ok(stdout)
    } else {
        let detail = [stdout, stderr]
            .into_iter()
            .filter(|s| !s.is_empty())
            .collect::<Vec<_>>()
            .join("\n");
        let hint = if detail.contains("401 Unauthorized") {
            "\n\nCodex authentication was rejected. Run `nanoharness login codex`, then retry."
        } else {
            ""
        };
        Err(format!(
            "{name} exited with {}{}{}",
            output.status,
            if detail.is_empty() { "" } else { ":\n" },
            detail
        ) + hint)
    }
}
fn codex(prompt: &str, model: Option<&str>, write: bool) -> Result<String, String> {
    let mut cmd = Command::new("codex");
    cmd.args([
        "exec",
        "--skip-git-repo-check",
        "--sandbox",
        if write {
            "workspace-write"
        } else {
            "read-only"
        },
    ]);
    if let Some(model) = model.filter(|m| !m.is_empty()) {
        cmd.args(["--model", model]);
    }
    cmd.arg("--").arg(prompt);
    command(cmd, "Codex")
}
fn pi(prompt: &str, model: Option<&str>) -> Result<String, String> {
    let mut cmd = Command::new("pi");
    cmd.arg("--print");
    if let Some(model) = model.filter(|m| !m.is_empty()) {
        cmd.args(["--model", model]);
    }
    cmd.arg("--").arg(prompt);
    command(cmd, "pi")
}
pub fn ask(
    provider: &str,
    prompt: &str,
    model: Option<&str>,
    write: bool,
) -> Result<String, String> {
    if prompt.trim().is_empty() {
        return Err("prompt is empty".into());
    }
    match provider {
        "codex" => codex(prompt, model, write),
        "openai" => openai(
            prompt,
            model.filter(|m| !m.is_empty()).unwrap_or("gpt-5-mini"),
        ),
        "anthropic" => anthropic(
            prompt,
            model
                .filter(|m| !m.is_empty())
                .unwrap_or("claude-sonnet-4-5"),
        ),
        "pi" => pi(prompt, model),
        _ => Err(format!("provider must be one of {}", NAMES.join(", "))),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn refuses_empty_prompts_before_starting_a_provider() {
        assert_eq!(
            ask("codex", "  ", None, false).unwrap_err(),
            "prompt is empty"
        );
    }

    #[test]
    fn reports_unknown_providers_without_starting_a_provider() {
        assert!(
            ask("unknown", "hello", None, false)
                .unwrap_err()
                .contains("codex")
        );
    }
}
