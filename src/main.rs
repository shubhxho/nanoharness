mod providers;
mod tui;

use std::{
    collections::BTreeMap,
    env, fs,
    io::Write,
    path::PathBuf,
    process::{Command, Stdio},
};

fn usage() {
    println!(
        "nanoharness\n\nUSAGE:\n  nanoharness                         Open the TUI\n  nanoharness tui                     Open the TUI\n  nanoharness login <codex|openai|anthropic|claude> [--api-key]\n  nanoharness run [--provider codex|openai|anthropic|pi] [--model MODEL] [--write] <PROMPT>"
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
fn save(name: &str, value: String) -> Result<(), String> {
    let path = credentials_path();
    let mut keys = credentials();
    keys.insert(name.into(), value);
    fs::create_dir_all(path.parent().unwrap()).map_err(|e| e.to_string())?;
    fs::write(
        &path,
        keys.into_iter()
            .map(|(k, v)| format!("{k}={v}\n"))
            .collect::<String>(),
    )
    .map_err(|e| e.to_string())?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(path, fs::Permissions::from_mode(0o600)).map_err(|e| e.to_string())?;
    }
    Ok(())
}
fn api_key() -> Result<String, String> {
    let key = rpassword::prompt_password("API key: ").map_err(|e| e.to_string())?;
    if key.is_empty() {
        Err("empty API key".into())
    } else {
        Ok(key)
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
fn login(kind: &str, args: &[String]) -> Result<(), String> {
    match kind {
        "codex" if args.is_empty() => vendor("codex", &["login"]),
        "codex" if args == ["--api-key"] => {
            let key = api_key()?;
            let mut child = Command::new("codex")
                .args(["login", "--with-api-key"])
                .stdin(Stdio::piped())
                .spawn()
                .map_err(|_| "Codex CLI not found".to_string())?;
            child
                .stdin
                .take()
                .unwrap()
                .write_all(key.as_bytes())
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
        "codex" => Err("use `login codex` or `login codex --api-key`".into()),
        _ => Err("login provider must be codex, openai, anthropic, or claude".into()),
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
        return Err("run needs a prompt; use `nanoharness` to open the TUI".into());
    } else {
        prompt.join(" ")
    };
    let provider = provider.unwrap_or_else(|| "codex".into());
    println!(
        "{}",
        providers::ask(&provider, &prompt, model.as_deref(), write)?
    );
    Ok(())
}
fn main() {
    let args: Vec<String> = env::args().skip(1).collect();
    let result = match args.first().map(String::as_str) {
        None | Some("tui") => tui::run(),
        Some("login") => args
            .get(1)
            .map(|p| login(p, &args[2..]))
            .unwrap_or_else(|| Err("login needs a provider".into())),
        Some("run") => run(&args[1..]),
        Some("-h" | "--help" | "help") => {
            usage();
            Ok(())
        }
        Some(x) => Err(format!(
            "unknown command: {x}; use `nanoharness` to open the TUI"
        )),
    };
    if let Err(error) = result {
        eprintln!("error: {error}");
        std::process::exit(1)
    }
}
