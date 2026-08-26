use reqwest::blocking::Client;
use serde_json::{Value, json};
use std::{
    collections::BTreeMap,
    env, fs,
    io::Write,
    path::PathBuf,
    process::{Command, Stdio},
};

fn usage() {
    println!(
        "nanoharness\n\nUSAGE:\n  nanoharness login <codex|openai|anthropic|claude> [--api-key]\n  nanoharness run [--provider codex|openai|anthropic] [--model MODEL] [--write] <PROMPT>"
    );
}

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

fn save(name: &str, value: String) -> Result<(), String> {
    let path = credentials_path();
    let mut keys = credentials();
    keys.insert(name.into(), value);
    fs::create_dir_all(path.parent().unwrap()).map_err(|e| e.to_string())?;
    let body = keys
        .into_iter()
        .map(|(k, v)| format!("{k}={v}\n"))
        .collect::<String>();
    fs::write(&path, body).map_err(|e| e.to_string())?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(path, fs::Permissions::from_mode(0o600)).map_err(|e| e.to_string())?;
    }
    Ok(())
}

fn api_key() -> Result<String, String> {
    let value = rpassword::prompt_password("API key: ").map_err(|e| e.to_string())?;
    if value.is_empty() {
        Err("empty API key".into())
    } else {
        Ok(value)
    }
}

fn login(kind: &str, args: &[String]) -> Result<(), String> {
    match kind {
        "codex" if args.is_empty() => vendor("codex", &["login"]),
        "codex" if args == ["--api-key"] => {
            let value = api_key()?;
            let mut child = Command::new("codex")
                .args(["login", "--with-api-key"])
                .stdin(Stdio::piped())
                .spawn()
                .map_err(|_| {
                    "Codex CLI not found. Install it from https://developers.openai.com/codex/cli"
                        .to_string()
                })?;
            child
                .stdin
                .take()
                .unwrap()
                .write_all(value.as_bytes())
                .map_err(|e| e.to_string())?;
            if child.wait().map_err(|e| e.to_string())?.success() {
                Ok(())
            } else {
                Err("Codex login failed".into())
            }
        }
        "openai" if args.is_empty() => save("OPENAI_API_KEY", api_key()?),
        "anthropic" if args.is_empty() => save("ANTHROPIC_API_KEY", api_key()?),
        "claude" if args.is_empty() => vendor("claude", &[]),
        "codex" => {
            Err("use `login codex` for browser login or `login codex --api-key` for a key".into())
        }
        _ => Err("login provider must be codex, openai, anthropic, or claude".into()),
    }
}

fn vendor(program: &str, args: &[&str]) -> Result<(), String> {
    let status = Command::new(program)
        .args(args)
        .status()
        .map_err(|_| format!("{program} CLI not found"))?;
    if status.success() {
        Ok(())
    } else {
        Err(format!("{program} login failed"))
    }
}

fn text(v: &[Value]) -> Option<&str> {
    v.iter().find_map(|x| x.get("text").and_then(Value::as_str))
}
fn fail(response: reqwest::blocking::Response) -> Result<Value, String> {
    let status = response.status();
    let body = response.text().unwrap_or_default();
    if status.is_success() {
        serde_json::from_str(&body).map_err(|e| e.to_string())
    } else {
        Err(format!("API returned {status}: {body}"))
    }
}

fn openai(prompt: &str, model: &str) -> Result<(), String> {
    let token =
        key("OPENAI_API_KEY").ok_or("missing OPENAI_API_KEY; run `nanoharness login openai`")?;
    let v = fail(
        Client::new()
            .post("https://api.openai.com/v1/responses")
            .bearer_auth(token)
            .json(&json!({"model": model, "input": prompt}))
            .send()
            .map_err(|e| e.to_string())?,
    )?;
    let answer = v
        .get("output")
        .and_then(Value::as_array)
        .and_then(|out| {
            out.iter().find_map(|item| {
                item.get("content")
                    .and_then(Value::as_array)
                    .and_then(|content| text(content))
            })
        })
        .ok_or_else(|| format!("no text in response: {v}"))?;
    println!("{answer}");
    Ok(())
}

fn anthropic(prompt: &str, model: &str) -> Result<(), String> {
    let token = key("ANTHROPIC_API_KEY")
        .ok_or("missing ANTHROPIC_API_KEY; run `nanoharness login anthropic`")?;
    let v = fail(Client::new().post("https://api.anthropic.com/v1/messages").header("x-api-key", token).header("anthropic-version", "2023-06-01").json(&json!({"model": model, "max_tokens": 4096, "messages": [{"role": "user", "content": prompt}]})).send().map_err(|e| e.to_string())?)?;
    println!(
        "{}",
        v.get("content")
            .and_then(Value::as_array)
            .and_then(|content| text(content))
            .ok_or_else(|| format!("no text in response: {v}"))?
    );
    Ok(())
}

fn codex(prompt: &str, model: Option<&str>, write: bool) -> Result<(), String> {
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
    if let Some(model) = model {
        cmd.args(["--model", model]);
    }
    let status = cmd.arg(prompt).status().map_err(|_| {
        "Codex CLI not found. Install it from https://developers.openai.com/codex/cli".to_string()
    })?;
    if status.success() {
        Ok(())
    } else {
        Err("Codex failed; run `nanoharness login codex` first".into())
    }
}

fn run(args: &[String]) -> Result<(), String> {
    let mut provider = None;
    let mut model = None;
    let mut write = false;
    let mut prompt = Vec::new();
    let mut i = 0;
    while i < args.len() {
        match args[i].as_str() {
            "--provider" => {
                i += 1;
                provider = args.get(i).cloned();
            }
            "--model" => {
                i += 1;
                model = args.get(i).cloned();
            }
            "--write" => write = true,
            x if x.starts_with('-') => return Err(format!("unknown option: {x}")),
            x => prompt.push(x.to_owned()),
        };
        i += 1;
    }
    let prompt = if prompt.is_empty() {
        return Err("run needs a prompt".into());
    } else {
        prompt.join(" ")
    };
    let provider = provider.unwrap_or_else(|| {
        if key("OPENAI_API_KEY").is_some() {
            "openai".into()
        } else if Command::new("codex")
            .args(["login", "status"])
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status()
            .is_ok_and(|s| s.success())
        {
            "codex".into()
        } else {
            "anthropic".into()
        }
    });
    match provider.as_str() {
        "codex" => codex(&prompt, model.as_deref(), write),
        "openai" => openai(&prompt, model.as_deref().unwrap_or("gpt-5-mini")),
        "anthropic" => anthropic(&prompt, model.as_deref().unwrap_or("claude-sonnet-4-5")),
        _ => Err("provider must be codex, openai, or anthropic".into()),
    }
}

fn main() {
    let args: Vec<String> = env::args().skip(1).collect();
    if matches!(
        args.first().map(String::as_str),
        Some("-h" | "--help" | "help") | None
    ) {
        usage();
        return;
    }
    let result = match args.first().map(String::as_str) {
        Some("login") => args
            .get(1)
            .map(|p| login(p, &args[2..]))
            .unwrap_or_else(|| Err("login needs a provider".into())),
        Some("run") => run(&args[1..]),
        Some(x) => Err(format!("unknown command: {x}")),
        None => Ok(()),
    };
    if let Err(e) = result {
        eprintln!("error: {e}");
        std::process::exit(1)
    }
}
