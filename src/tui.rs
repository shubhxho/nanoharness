use crate::providers::{NAMES, ask};
use crossterm::{
    event::{self, Event, KeyCode, KeyEventKind, KeyModifiers},
    execute,
    terminal::{EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode},
};
use ratatui::{
    Terminal,
    backend::CrosstermBackend,
    layout::{Constraint, Direction, Layout},
    style::{Color, Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, Paragraph, Wrap},
};
use std::{
    io,
    process::{Command, Stdio},
    sync::mpsc::{self, Receiver},
    thread,
    time::Duration,
};

struct Message {
    who: &'static str,
    text: String,
    error: bool,
}
struct App {
    provider: usize,
    model: String,
    input: String,
    messages: Vec<Message>,
    status: String,
    write: bool,
    pending: Option<Receiver<Result<String, String>>>,
    auth: String,
}
impl App {
    fn new() -> Self {
        let ready = Command::new("codex")
            .args(["login", "status"])
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status()
            .is_ok_and(|s| s.success());
        Self { provider: 0, model: String::new(), input: String::new(), messages: vec![Message { who: "nano", text: "Ready. Type a request and press Enter. Tab changes backend; /help lists commands.".into(), error: false }], status: "idle".into(), write: false, pending: None, auth: if ready { "Codex auth ready".into() } else { "Codex login needed".into() } }
    }
    fn name(&self) -> &'static str {
        NAMES[self.provider]
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
        let provider = self.name();
        let model = self.model.clone();
        let write = self.write;
        self.messages.push(Message {
            who: "you",
            text: prompt.clone(),
            error: false,
        });
        self.status = format!("running {provider}…");
        let (tx, rx) = mpsc::channel();
        self.pending = Some(rx);
        thread::spawn(move || {
            let _ = tx.send(ask(
                provider,
                &prompt,
                (!model.is_empty()).then_some(model.as_str()),
                write,
            ));
        });
    }
    fn command(&mut self, command: &str) {
        let mut parts = command.splitn(2, char::is_whitespace);
        let name = parts.next().unwrap_or_default();
        let value = parts.next().unwrap_or_default().trim();
        match name {
            "/help" => self.messages.push(Message { who: "nano", text: "Tab backend · Ctrl+W write toggle · /provider <codex|openai|anthropic|pi> · /model <name> · /clear · /exit".into(), error: false }),
            "/clear" => self.messages.clear(),
            "/exit" => self.status = "quit".into(),
            "/write" => { self.write = !self.write; self.status = format!("Codex write access: {}", if self.write { "on" } else { "off" }); },
            "/model" if !value.is_empty() => { self.model = value.into(); self.status = "model updated".into(); },
            "/provider" => match NAMES.iter().position(|p| *p == value) { Some(i) => { self.provider = i; self.status = format!("backend: {}", self.name()); }, None => self.status = format!("unknown backend: {value}"), },
            _ => self.status = "unknown command; use /help".into(),
        }
    }
    fn collect(&mut self) {
        if let Some(rx) = &self.pending
            && let Ok(result) = rx.try_recv()
        {
            match result {
                Ok(text) => self.messages.push(Message {
                    who: self.name(),
                    text: if text.is_empty() {
                        "(no text returned)".into()
                    } else {
                        text
                    },
                    error: false,
                }),
                Err(error) => self.messages.push(Message {
                    who: "error",
                    text: error,
                    error: true,
                }),
            };
            self.pending = None;
            self.status = "idle".into();
        }
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
        match Terminal::new(CrosstermBackend::new(stdout)) {
            Ok(terminal) => Ok(Self { terminal }),
            Err(error) => {
                let _ = disable_raw_mode();
                Err(error.to_string())
            }
        }
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
            if app.pending.is_some() {
                continue;
            }
            match key.code {
                KeyCode::Enter => app.submit(),
                KeyCode::Backspace => {
                    app.input.pop();
                }
                KeyCode::Esc => app.input.clear(),
                KeyCode::Tab => {
                    app.provider = (app.provider + 1) % NAMES.len();
                    app.status = format!("backend: {}", app.name());
                }
                KeyCode::Char('q') if app.input.is_empty() => return Ok(()),
                KeyCode::Char('w') if key.modifiers.contains(KeyModifiers::CONTROL) => {
                    app.command("/write")
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
    let area = frame.area();
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(3),
            Constraint::Min(5),
            Constraint::Length(3),
            Constraint::Length(1),
        ])
        .split(area);
    let title = format!(
        " nano harness  ·  {}  ·  {} ",
        app.name(),
        if app.model.is_empty() {
            "vendor default"
        } else {
            &app.model
        }
    );
    frame.render_widget(
        Paragraph::new(Line::from(vec![
            Span::styled(
                title,
                Style::default()
                    .fg(Color::Cyan)
                    .add_modifier(Modifier::BOLD),
            ),
            Span::raw(format!("   {}", app.auth)),
        ]))
        .block(Block::default().borders(Borders::ALL)),
        chunks[0],
    );
    let lines: Vec<Line> = app
        .messages
        .iter()
        .rev()
        .take(24)
        .rev()
        .flat_map(|m| {
            let style = if m.error {
                Style::default().fg(Color::Red)
            } else if m.who == "you" {
                Style::default().fg(Color::Green)
            } else {
                Style::default().fg(Color::Yellow)
            };
            vec![
                Line::from(Span::styled(
                    format!("{}  ", m.who),
                    style.add_modifier(Modifier::BOLD),
                )),
                Line::from(m.text.clone()),
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
    let prompt = if app.pending.is_some() {
        "Waiting for backend…".to_owned()
    } else {
        app.input.clone()
    };
    frame.render_widget(
        Paragraph::new(prompt)
            .wrap(Wrap { trim: false })
            .block(Block::default().borders(Borders::ALL).title(" prompt ")),
        chunks[2],
    );
    frame.render_widget(
        Paragraph::new(format!(
            " {}   Enter send · Tab backend · Ctrl+W write · /help · Ctrl+C quit",
            app.status
        ))
        .style(Style::default().fg(Color::DarkGray)),
        chunks[3],
    );
}
