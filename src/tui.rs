use crate::providers::{PROFILES, ask, auth_status};
use crossterm::{
    event::{self, Event, KeyCode, KeyEvent, KeyEventKind, KeyModifiers},
    execute,
    terminal::{EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode},
};
use ratatui::{
    Terminal,
    backend::CrosstermBackend,
    layout::{Alignment, Constraint, Direction, Layout, Rect},
    style::{Color, Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, Clear, Paragraph, Wrap},
};
use serde::{Deserialize, Serialize};
use std::{
    env, fs, io,
    path::PathBuf,
    process,
    sync::mpsc::{self, Receiver},
    thread,
    time::Duration,
};

#[derive(Clone, Serialize, Deserialize)]
struct Message {
    who: String,
    text: String,
    error: bool,
}
#[derive(Default, Serialize, Deserialize)]
struct Saved {
    provider: String,
    models: Vec<String>,
    messages: Vec<Message>,
    write: bool,
}
#[derive(Clone)]
struct Request {
    provider: String,
    model: String,
    prompt: String,
    write: bool,
}
enum Overlay {
    Provider(usize),
    Model(usize),
    Confirm(Request),
    Detail(String),
}
struct App {
    provider: usize,
    models: Vec<String>,
    input: String,
    messages: Vec<Message>,
    status: String,
    write: bool,
    pending: Option<Receiver<(Request, Result<String, String>)>>,
    overlay: Option<Overlay>,
    auth: String,
    scroll: usize,
}

fn state_path() -> PathBuf {
    env::var_os("XDG_STATE_HOME")
        .map(PathBuf::from)
        .or_else(|| env::var_os("HOME").map(|home| PathBuf::from(home).join(".local/state")))
        .unwrap_or_else(|| PathBuf::from("."))
        .join("nanoharness/session.json")
}
fn clean(text: String) -> String {
    text.chars()
        .filter(|c| *c == '\n' || *c == '\t' || !c.is_control())
        .collect()
}
impl App {
    fn new() -> Self {
        let saved: Saved = fs::read_to_string(state_path())
            .ok()
            .and_then(|text| serde_json::from_str(&text).ok())
            .unwrap_or_default();
        let provider = PROFILES
            .iter()
            .position(|item| item.id == saved.provider)
            .unwrap_or(0);
        let models = if saved.models.len() == PROFILES.len() {
            saved.models
        } else {
            PROFILES
                .iter()
                .map(|item| item.default_model.into())
                .collect()
        };
        let messages = if saved.messages.is_empty() {
            vec![Message {
                who: "nano".into(),
                text: "Ready. Press p for provider, m for model, or type a request.".into(),
                error: false,
            }]
        } else {
            saved.messages
        };
        let mut app = Self {
            provider,
            models,
            input: String::new(),
            messages,
            status: "idle".into(),
            write: false,
            pending: None,
            overlay: None,
            auth: String::new(),
            scroll: 0,
        };
        app.refresh_auth();
        app
    }
    fn profile(&self) -> &'static crate::providers::Profile {
        &PROFILES[self.provider]
    }
    fn model(&self) -> &str {
        &self.models[self.provider]
    }
    fn refresh_auth(&mut self) {
        self.auth = auth_status(self.profile().id);
    }
    fn persist(&self) {
        let saved = Saved {
            provider: self.profile().id.into(),
            models: self.models.clone(),
            messages: self
                .messages
                .iter()
                .rev()
                .take(200)
                .cloned()
                .collect::<Vec<_>>()
                .into_iter()
                .rev()
                .collect(),
            write: false,
        };
        let path = state_path();
        if let Some(parent) = path.parent() {
            let _ = fs::create_dir_all(parent);
        }
        if let Ok(text) = serde_json::to_string(&saved) {
            let temporary = path.with_extension(format!("{}.tmp", process::id()));
            if fs::write(&temporary, text).is_ok() {
                let _ = fs::rename(temporary, path);
            }
        }
    }
    fn push(&mut self, who: &str, text: String, error: bool) {
        self.messages.push(Message {
            who: who.into(),
            text: clean(text),
            error,
        });
        if self.messages.len() > 200 {
            self.messages.remove(0);
        }
        self.scroll = 0;
        self.persist();
    }
    fn request(&self, prompt: String) -> Request {
        Request {
            provider: self.profile().id.into(),
            model: self.model().into(),
            prompt,
            write: self.write && self.profile().id == "codex",
        }
    }
    fn start(&mut self, request: Request) {
        self.status = format!("running {}…", request.provider);
        let (tx, rx) = mpsc::channel();
        self.pending = Some(rx);
        thread::spawn(move || {
            let result = ask(
                &request.provider,
                &request.prompt,
                (!request.model.is_empty()).then_some(request.model.as_str()),
                request.write,
            );
            let _ = tx.send((request, result));
        });
    }
    fn submit(&mut self) {
        let prompt = self.input.trim().to_owned();
        self.input.clear();
        if prompt.is_empty() || self.pending.is_some() {
            return;
        }
        if prompt.starts_with('/') {
            self.command(&prompt);
            return;
        }
        self.push("you", prompt.clone(), false);
        let request = self.request(prompt);
        if request.write {
            self.overlay = Some(Overlay::Confirm(request));
            self.status = "confirm workspace write".into();
        } else {
            self.start(request);
        }
    }
    fn command(&mut self, command: &str) {
        let mut parts = command.splitn(2, char::is_whitespace);
        let name = parts.next().unwrap_or_default();
        let value = parts.next().unwrap_or_default().trim();
        match name {
            "/help" => self.push("nano", "p provider picker · m model picker · Enter send · ↑/↓ scroll · Ctrl+W write · /provider NAME · /model NAME · /new · /status · /exit".into(), false),
            "/new" | "/clear" => { self.messages.clear(); self.status = "new conversation".into(); self.persist(); },
            "/exit" => self.status = "quit".into(),
            "/status" => { self.refresh_auth(); self.status = self.auth.clone(); },
            "/write" => self.toggle_write(),
            "/provider" => match PROFILES.iter().position(|item| item.id == value) { Some(index) => self.select_provider(index), None => self.status = format!("unknown provider: {value}"), },
            "/model" if !value.is_empty() => { self.models[self.provider] = value.into(); self.status = "custom model selected".into(); self.persist(); },
            _ => self.status = "unknown command; use /help".into(),
        }
    }
    fn select_provider(&mut self, index: usize) {
        self.provider = index;
        self.refresh_auth();
        self.status = format!("{}: {}", self.profile().label, self.auth);
        self.persist();
    }
    fn toggle_write(&mut self) {
        if self.profile().id != "codex" {
            self.status = "write mode only applies to Codex".into();
        } else {
            self.write = !self.write;
            self.status = format!(
                "workspace write {}",
                if self.write {
                    "armed; confirmation required"
                } else {
                    "off"
                }
            );
            self.persist();
        }
    }
    fn collect(&mut self) {
        if let Some(rx) = &self.pending
            && let Ok((request, result)) = rx.try_recv()
        {
            match result {
                Ok(text) => self.push(
                    &request.provider,
                    if text.trim().is_empty() {
                        "(no text returned)".into()
                    } else {
                        text
                    },
                    false,
                ),
                Err(error) => self.push("error", error, true),
            };
            if request.write {
                self.write = false;
            }
            self.pending = None;
            self.status = "idle".into();
            self.persist();
        }
    }
    fn handle_overlay(&mut self, key: KeyEvent) -> bool {
        let Some(overlay) = self.overlay.take() else {
            return false;
        };
        match overlay {
            Overlay::Provider(mut selected) => match key.code {
                KeyCode::Esc => {}
                KeyCode::Up | KeyCode::Char('k') => {
                    selected = selected.saturating_sub(1);
                    self.overlay = Some(Overlay::Provider(selected));
                }
                KeyCode::Down | KeyCode::Char('j') => {
                    selected = (selected + 1).min(PROFILES.len() - 1);
                    self.overlay = Some(Overlay::Provider(selected));
                }
                KeyCode::Enter => self.select_provider(selected),
                _ => self.overlay = Some(Overlay::Provider(selected)),
            },
            Overlay::Model(mut selected) => {
                let models = self.profile().models;
                match key.code {
                    KeyCode::Esc => {}
                    KeyCode::Up | KeyCode::Char('k') => {
                        selected = selected.saturating_sub(1);
                        self.overlay = Some(Overlay::Model(selected));
                    }
                    KeyCode::Down | KeyCode::Char('j') => {
                        selected = (selected + 1).min(models.len() - 1);
                        self.overlay = Some(Overlay::Model(selected));
                    }
                    KeyCode::Enter => {
                        self.models[self.provider] = models[selected].into();
                        self.status = "model selected".into();
                        self.persist();
                    }
                    _ => self.overlay = Some(Overlay::Model(selected)),
                }
            }
            Overlay::Confirm(request) => match key.code {
                KeyCode::Enter | KeyCode::Char('y') => self.start(request),
                _ => {
                    self.write = false;
                    self.status = "write request cancelled".into();
                    self.persist();
                }
            },
            Overlay::Detail(_) => {}
        }
        true
    }
}

struct Screen {
    terminal: Terminal<CrosstermBackend<io::Stdout>>,
}
impl Screen {
    fn enter() -> Result<Self, String> {
        enable_raw_mode().map_err(|e| e.to_string())?;
        let mut stdout = io::stdout();
        if let Err(error) = execute!(stdout, EnterAlternateScreen) {
            let _ = disable_raw_mode();
            return Err(error.to_string());
        }
        Terminal::new(CrosstermBackend::new(stdout))
            .map(|terminal| Self { terminal })
            .map_err(|e| e.to_string())
    }
}
impl Drop for Screen {
    fn drop(&mut self) {
        let _ = disable_raw_mode();
        let _ = execute!(self.terminal.backend_mut(), LeaveAlternateScreen);
        let _ = self.terminal.show_cursor();
    }
}

pub fn run() -> Result<(), String> {
    let mut screen = Screen::enter()?;
    let mut app = App::new();
    app.persist();
    loop {
        app.collect();
        screen
            .terminal
            .draw(|frame| draw(frame, &app))
            .map_err(|e| e.to_string())?;
        if app.status == "quit" {
            return Ok(());
        }
        if event::poll(Duration::from_millis(80)).map_err(|e| e.to_string())?
            && let Event::Key(key) = event::read().map_err(|e| e.to_string())?
        {
            if key.kind != KeyEventKind::Press {
                continue;
            }
            if key.modifiers.contains(KeyModifiers::CONTROL)
                && matches!(key.code, KeyCode::Char('c'))
            {
                return Ok(());
            }
            if app.overlay.is_some() {
                app.handle_overlay(key);
                continue;
            }
            if app.pending.is_some() {
                continue;
            }
            match key.code {
                KeyCode::Enter => app.submit(),
                KeyCode::Backspace => {
                    app.input.pop();
                }
                KeyCode::Esc => app.input.clear(),
                KeyCode::Char('q') if app.input.is_empty() => return Ok(()),
                KeyCode::Tab => app.select_provider((app.provider + 1) % PROFILES.len()),
                KeyCode::Char('p') if app.input.is_empty() => {
                    app.overlay = Some(Overlay::Provider(app.provider))
                }
                KeyCode::Char('m') if app.input.is_empty() => {
                    let selected = app
                        .profile()
                        .models
                        .iter()
                        .position(|model| *model == app.model())
                        .unwrap_or(0);
                    app.overlay = Some(Overlay::Model(selected));
                }
                KeyCode::Up => {
                    app.scroll = (app.scroll + 1).min(app.messages.len().saturating_sub(1));
                }
                KeyCode::Down => {
                    app.scroll = app.scroll.saturating_sub(1);
                }
                KeyCode::Home => app.scroll = app.messages.len().saturating_sub(1),
                KeyCode::End => app.scroll = 0,
                KeyCode::Char('w') if key.modifiers.contains(KeyModifiers::CONTROL) => {
                    app.toggle_write()
                }
                KeyCode::Char('e') if app.input.is_empty() => {
                    if let Some(message) = app.messages.iter().rev().find(|message| message.error) {
                        app.overlay = Some(Overlay::Detail(message.text.clone()));
                    }
                }
                KeyCode::Char(c) if !key.modifiers.contains(KeyModifiers::CONTROL) => {
                    app.input.push(c)
                }
                _ => {}
            }
        }
    }
}

fn draw(frame: &mut ratatui::Frame, app: &App) {
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(3),
            Constraint::Min(5),
            Constraint::Length(3),
            Constraint::Length(1),
        ])
        .split(frame.area());
    let model = if app.model().is_empty() {
        "vendor default"
    } else {
        app.model()
    };
    let mode = if app.write {
        "WRITE ARMED"
    } else {
        "read-only"
    };
    frame.render_widget(
        Paragraph::new(Line::from(vec![
            Span::styled(
                format!(" nano  {} ", app.profile().label),
                Style::default()
                    .fg(Color::Cyan)
                    .add_modifier(Modifier::BOLD),
            ),
            Span::raw(format!(" model: {model}  ·  {}  ·  {mode}", app.auth)),
        ]))
        .block(Block::default().borders(Borders::ALL)),
        chunks[0],
    );
    let lines: Vec<Line> = app
        .messages
        .iter()
        .rev()
        .skip(app.scroll)
        .take(30)
        .rev()
        .flat_map(|message| {
            let style = if message.error {
                Style::default().fg(Color::Red)
            } else if message.who == "you" {
                Style::default().fg(Color::Green)
            } else {
                Style::default().fg(Color::Yellow)
            };
            vec![
                Line::from(Span::styled(
                    format!("{}  ", message.who),
                    style.add_modifier(Modifier::BOLD),
                )),
                Line::from(message.text.clone()),
                Line::raw(""),
            ]
        })
        .collect();
    frame.render_widget(
        Paragraph::new(lines).wrap(Wrap { trim: false }).block(
            Block::default()
                .borders(Borders::ALL)
                .title(" conversation "),
        ),
        chunks[1],
    );
    let input = if app.pending.is_some() {
        "Waiting for backend…".to_owned()
    } else {
        app.input.clone()
    };
    frame.render_widget(
        Paragraph::new(input)
            .wrap(Wrap { trim: false })
            .block(Block::default().borders(Borders::ALL).title(" prompt ")),
        chunks[2],
    );
    frame.render_widget(
        Paragraph::new(format!(
            " {}   p provider · m model · ↑↓ scroll · Ctrl+W write · /help · q quit",
            app.status
        ))
        .style(Style::default().fg(Color::DarkGray)),
        chunks[3],
    );
    if let Some(overlay) = &app.overlay {
        draw_overlay(frame, overlay, app);
    }
}
fn draw_overlay(frame: &mut ratatui::Frame, overlay: &Overlay, app: &App) {
    let area = centered(64, 60, frame.area());
    let (title, lines) = match overlay {
        Overlay::Provider(selected) => (
            " provider ",
            PROFILES
                .iter()
                .enumerate()
                .map(|(index, item)| {
                    Line::from(if index == *selected {
                        format!("> {}  — {}", item.label, auth_status(item.id))
                    } else {
                        format!("  {}", item.label)
                    })
                })
                .collect(),
        ),
        Overlay::Model(selected) => (
            " model ",
            app.profile()
                .models
                .iter()
                .enumerate()
                .map(|(index, model)| {
                    Line::from(if index == *selected {
                        format!(
                            "> {}",
                            if model.is_empty() {
                                "vendor default"
                            } else {
                                model
                            }
                        )
                    } else {
                        format!(
                            "  {}",
                            if model.is_empty() {
                                "vendor default"
                            } else {
                                model
                            }
                        )
                    })
                })
                .collect(),
        ),
        Overlay::Confirm(request) => (
            " confirm write ",
            vec![
                Line::from("Codex will receive workspace-write access for this request:"),
                Line::raw(""),
                Line::from(request.prompt.clone()),
                Line::raw(""),
                Line::from("Enter or y runs it. Any other key cancels."),
            ],
        ),
        Overlay::Detail(detail) => (
            " latest error ",
            detail
                .lines()
                .take(16)
                .map(|line| Line::from(line.to_owned()))
                .collect(),
        ),
    };
    frame.render_widget(Clear, area);
    frame.render_widget(
        Paragraph::new(lines)
            .alignment(Alignment::Left)
            .wrap(Wrap { trim: false })
            .block(Block::default().borders(Borders::ALL).title(title)),
        area,
    );
}
fn centered(width: u16, height: u16, area: Rect) -> Rect {
    let vertical = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Percentage((100 - height) / 2),
            Constraint::Percentage(height),
            Constraint::Percentage((100 - height) / 2),
        ])
        .split(area);
    Layout::default()
        .direction(Direction::Horizontal)
        .constraints([
            Constraint::Percentage((100 - width) / 2),
            Constraint::Percentage(width),
            Constraint::Percentage((100 - width) / 2),
        ])
        .split(vertical[1])[1]
}
