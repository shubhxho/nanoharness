use crate::{
    context,
    providers::{PROFILES, ask, auth_status},
};
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
    widgets::{Block, BorderType, Borders, Clear, Padding, Paragraph, Wrap},
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
use unicode_width::UnicodeWidthStr;

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
    citations: usize,
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
    cursor: usize,
    messages: Vec<Message>,
    status: String,
    write: bool,
    pending: Option<Receiver<(Request, Result<String, String>)>>,
    overlay: Option<Overlay>,
    auth: String,
    auths: Vec<String>,
    scroll: usize,
    context_enabled: bool,
    citations: Vec<context::Citation>,
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
            cursor: 0,
            messages,
            status: "idle".into(),
            write: false,
            pending: None,
            overlay: None,
            auth: String::new(),
            auths: PROFILES.iter().map(|item| auth_status(item.id)).collect(),
            scroll: 0,
            context_enabled: false,
            citations: Vec::new(),
        };
        app.auth = app.auths[app.provider].clone();
        app
    }
    fn profile(&self) -> &'static crate::providers::Profile {
        &PROFILES[self.provider]
    }
    fn model(&self) -> &str {
        &self.models[self.provider]
    }
    fn refresh_auth(&mut self) {
        self.auths[self.provider] = auth_status(self.profile().id);
        self.auth = self.auths[self.provider].clone();
    }
    fn input_len(&self) -> usize {
        self.input.chars().count()
    }
    fn byte_at(&self, position: usize) -> usize {
        self.input
            .char_indices()
            .nth(position)
            .map(|(byte, _)| byte)
            .unwrap_or(self.input.len())
    }
    fn insert(&mut self, text: &str) {
        let byte = self.byte_at(self.cursor);
        self.input.insert_str(byte, text);
        self.cursor += text.chars().count();
    }
    fn backspace(&mut self) {
        if self.cursor > 0 {
            let end = self.byte_at(self.cursor);
            let start = self.byte_at(self.cursor - 1);
            self.input.replace_range(start..end, "");
            self.cursor -= 1;
        }
    }
    fn delete(&mut self) {
        if self.cursor < self.input_len() {
            let start = self.byte_at(self.cursor);
            let end = self.byte_at(self.cursor + 1);
            self.input.replace_range(start..end, "");
        }
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
        let citations = if self.context_enabled {
            self.citations.len()
        } else {
            0
        };
        let prompt = if citations == 0 {
            prompt
        } else {
            format!("{prompt}\n\n{}", context::render_context(&self.citations))
        };
        Request {
            provider: self.profile().id.into(),
            model: self.model().into(),
            prompt,
            write: self.write && self.profile().id == "codex",
            citations,
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
        self.cursor = 0;
        if prompt.is_empty() || self.pending.is_some() {
            return;
        }
        if prompt.starts_with('/') {
            self.command(&prompt);
            return;
        }
        self.push("you", prompt.clone(), false);
        let request = self.request(prompt);
        if request.write || request.citations > 0 {
            self.overlay = Some(Overlay::Confirm(request));
            self.status = if self.write {
                "confirm workspace write".into()
            } else {
                "confirm context attachment".into()
            };
        } else {
            self.start(request);
        }
    }
    fn command(&mut self, command: &str) {
        let mut parts = command.splitn(2, char::is_whitespace);
        let name = parts.next().unwrap_or_default();
        let value = parts.next().unwrap_or_default().trim();
        match name {
            "/help" => self.push("nano", "Enter sends · F2/Ctrl+P provider · F3 model · Tab next provider · PageUp/PageDown scroll · Ctrl+W write · /context on|off · /query TERMS · /research QUESTION · /impact SYMBOL".into(), false),
            "/new" | "/clear" => { self.messages.clear(); self.status = "new conversation".into(); self.persist(); },
            "/exit" => self.status = "quit".into(),
            "/status" => { self.refresh_auth(); self.status = self.auth.clone(); },
            "/write" => self.toggle_write(),
            "/provider" => match PROFILES.iter().position(|item| item.id == value) { Some(index) => self.select_provider(index), None => self.status = format!("unknown provider: {value}"), },
            "/model" if !value.is_empty() => { self.models[self.provider] = value.into(); self.status = "custom model selected".into(); self.persist(); },
            "/context" => match value { "on" => { self.context_enabled = true; self.status = if self.citations.is_empty() { "context is on; use /query first".into() } else { format!("context on: {} citations will be attached after confirmation", self.citations.len()) }; }, "off" => { self.context_enabled = false; self.status = "context is off".into(); }, "clear" => { self.citations.clear(); self.status = "context citations cleared".into(); }, "status" | "" => self.status = format!("local lexical context: {} · {} citations", if self.context_enabled { "on" } else { "off" }, self.citations.len()), _ => self.status = "use /context on, off, clear, or status".into(), },
            "/query" | "/research" | "/impact" if !value.is_empty() => self.search_context(name, value),
            _ => self.status = "unknown command; use /help".into(),
        }
    }
    fn search_context(&mut self, mode: &str, query: &str) {
        match context::working_root().and_then(|root| context::search(&root, query)) {
            Ok(mut report) => {
                report.citations.truncate(8);
                self.citations = report.citations;
                let heading = match mode {
                    "/research" => "LOCAL LEXICAL EVIDENCE",
                    "/impact" => "POSSIBLE LEXICAL IMPACT",
                    _ => "LOCAL LEXICAL CONTEXT",
                };
                let results = if self.citations.is_empty() {
                    "No local matches.".into()
                } else {
                    self.citations
                        .iter()
                        .enumerate()
                        .map(|(index, citation)| {
                            format!(
                                "{:02}  {}:{}-{}",
                                index + 1,
                                citation.path,
                                citation.start_line,
                                citation.end_line
                            )
                        })
                        .collect::<Vec<_>>()
                        .join("\n")
                };
                self.push("context", format!("{heading}\nExact token/path matching only; no embeddings or dependency graph.\nquery: {query}\n\n{results}"), false);
                self.status = format!("{} local citations ready", self.citations.len());
            }
            Err(error) => self.push(
                "error",
                format!("local context search failed: {error}"),
                true,
            ),
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
                KeyCode::Esc | KeyCode::Char('n') => {
                    if request.write {
                        self.write = false;
                    }
                    self.status = "request cancelled; local citations remain private".into();
                    self.persist();
                }
                _ => self.overlay = Some(Overlay::Confirm(request)),
            },
            Overlay::Detail(detail) => match key.code {
                KeyCode::Esc | KeyCode::Enter | KeyCode::Char('q') => {}
                _ => self.overlay = Some(Overlay::Detail(detail)),
            },
        }
        true
    }
}

struct Screen {
    terminal: Terminal<CrosstermBackend<io::Stdout>>,
}
impl Screen {
    fn enter() -> Result<Self, String> {
        let stdout = io::stdout();
        let mut terminal =
            Terminal::new(CrosstermBackend::new(stdout)).map_err(|e| e.to_string())?;
        enable_raw_mode().map_err(|e| e.to_string())?;
        if let Err(error) = execute!(terminal.backend_mut(), EnterAlternateScreen) {
            let _ = disable_raw_mode();
            return Err(error.to_string());
        }
        Ok(Self { terminal })
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
        if !event::poll(Duration::from_millis(80)).map_err(|e| e.to_string())? {
            continue;
        }
        match event::read().map_err(|e| e.to_string())? {
            Event::Paste(text) if app.overlay.is_none() && app.pending.is_none() => {
                app.insert(&text)
            }
            Event::Key(key) if key.kind == KeyEventKind::Press => {
                if key.modifiers.contains(KeyModifiers::CONTROL)
                    && matches!(key.code, KeyCode::Char('c'))
                {
                    return Ok(());
                }
                if app.overlay.is_some() {
                    app.handle_overlay(key);
                    continue;
                }
                if key.modifiers.contains(KeyModifiers::CONTROL)
                    && matches!(key.code, KeyCode::Char('p'))
                {
                    app.overlay = Some(Overlay::Provider(app.provider));
                    continue;
                }
                if matches!(key.code, KeyCode::F(2)) {
                    app.overlay = Some(Overlay::Provider(app.provider));
                    continue;
                }
                if matches!(key.code, KeyCode::F(3)) {
                    let selected = app
                        .profile()
                        .models
                        .iter()
                        .position(|model| *model == app.model())
                        .unwrap_or(0);
                    app.overlay = Some(Overlay::Model(selected));
                    continue;
                }
                if matches!(key.code, KeyCode::F(4)) {
                    app.context_enabled = !app.context_enabled;
                    app.status = format!(
                        "local context {}",
                        if app.context_enabled { "on" } else { "off" }
                    );
                    continue;
                }
                if key.modifiers.contains(KeyModifiers::CONTROL)
                    && matches!(key.code, KeyCode::Char('e'))
                {
                    if let Some(message) = app.messages.iter().rev().find(|message| message.error) {
                        app.overlay = Some(Overlay::Detail(message.text.clone()));
                    }
                    continue;
                }
                if matches!(key.code, KeyCode::F(1)) {
                    app.overlay = Some(Overlay::Detail("COMMAND PALETTE\n\nF2 / Ctrl+P  provider picker\nF3  model picker\nF4  toggle local context\nTab  next provider\nCtrl+W  arm Codex write mode\nPageUp / PageDown  browse transcript\n/model NAME  custom model\n/provider NAME  choose backend\n/new  fresh transcript\n/status  refresh readiness\nCtrl+C  quit".into()));
                    continue;
                }
                match key.code {
                    KeyCode::PageUp | KeyCode::Up => {
                        app.scroll = (app.scroll + 5).min(app.messages.len().saturating_sub(1))
                    }
                    KeyCode::PageDown | KeyCode::Down => app.scroll = app.scroll.saturating_sub(5),
                    KeyCode::Tab if app.pending.is_none() => {
                        app.select_provider((app.provider + 1) % PROFILES.len())
                    }
                    KeyCode::Enter if app.pending.is_none() => app.submit(),
                    KeyCode::Backspace if app.pending.is_none() => app.backspace(),
                    KeyCode::Delete if app.pending.is_none() => app.delete(),
                    KeyCode::Left if app.pending.is_none() => {
                        app.cursor = app.cursor.saturating_sub(1)
                    }
                    KeyCode::Right if app.pending.is_none() => {
                        app.cursor = (app.cursor + 1).min(app.input_len())
                    }
                    KeyCode::Home if app.pending.is_none() => app.cursor = 0,
                    KeyCode::End if app.pending.is_none() => app.cursor = app.input_len(),
                    KeyCode::Esc if app.pending.is_none() => {
                        app.input.clear();
                        app.cursor = 0;
                    }
                    KeyCode::Char('w')
                        if key.modifiers.contains(KeyModifiers::CONTROL)
                            && app.pending.is_none() =>
                    {
                        app.toggle_write()
                    }
                    KeyCode::Char(c)
                        if !key.modifiers.contains(KeyModifiers::CONTROL)
                            && app.pending.is_none() =>
                    {
                        app.insert(&c.to_string())
                    }
                    _ => {}
                }
            }
            _ => {}
        }
    }
}

const BG: Color = Color::Rgb(30, 30, 46);
const SURFACE: Color = Color::Rgb(49, 50, 68);
const OVERLAY: Color = Color::Rgb(35, 36, 52);
const BORDER: Color = Color::Rgb(69, 71, 90);
const TEXT: Color = Color::Rgb(205, 214, 244);
const MUTED: Color = Color::Rgb(147, 153, 178);
const LAVENDER: Color = Color::Rgb(180, 190, 254);
const TEAL: Color = Color::Rgb(148, 226, 213);
const PEACH: Color = Color::Rgb(250, 179, 135);
const RED: Color = Color::Rgb(243, 139, 168);
const YELLOW: Color = Color::Rgb(249, 226, 175);

fn chip(text: impl Into<String>, foreground: Color, background: Color) -> Span<'static> {
    Span::styled(
        format!(" {} ", text.into()),
        Style::default()
            .fg(foreground)
            .bg(background)
            .add_modifier(Modifier::BOLD),
    )
}
fn panel(title: &'static str) -> Block<'static> {
    Block::default()
        .title(Span::styled(
            title,
            Style::default().fg(MUTED).add_modifier(Modifier::BOLD),
        ))
        .borders(Borders::ALL)
        .border_type(BorderType::Rounded)
        .border_style(Style::default().fg(BORDER))
        .style(Style::default().bg(SURFACE))
        .padding(Padding::new(1, 1, 0, 0))
}
fn draw(frame: &mut ratatui::Frame, app: &App) {
    let area = frame.area();
    frame.render_widget(Block::default().style(Style::default().bg(BG)), area);
    if area.width < 54 || area.height < 12 {
        frame.render_widget(Paragraph::new("✦ nano needs at least 54 × 12 terminal cells\n\nResize this terminal, then continue.").alignment(Alignment::Center).style(Style::default().fg(TEXT).bg(BG)), area);
        return;
    }
    let layout = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(1),
            Constraint::Min(5),
            Constraint::Length(4),
            Constraint::Length(1),
        ])
        .split(area);
    draw_header(frame, layout[0], app);
    let content = if area.width >= 92 {
        Layout::default()
            .direction(Direction::Horizontal)
            .constraints([Constraint::Min(48), Constraint::Length(28)])
            .split(layout[1])
    } else {
        Layout::default()
            .direction(Direction::Horizontal)
            .constraints([Constraint::Min(1)])
            .split(layout[1])
    };
    draw_conversation(frame, content[0], app);
    if content.len() > 1 {
        draw_sidebar(frame, content[1], app);
    }
    draw_composer(frame, layout[2], app);
    let error_hint = if app.messages.iter().any(|message| message.error) {
        "  Ctrl+E error"
    } else {
        ""
    };
    frame.render_widget(
        Paragraph::new(Line::from(vec![
            chip("Enter", TEXT, SURFACE),
            Span::styled(" send   ", Style::default().fg(MUTED)),
            chip("F2", TEXT, SURFACE),
            Span::styled(" provider   ", Style::default().fg(MUTED)),
            chip("F3", TEXT, SURFACE),
            Span::styled(" model   ", Style::default().fg(MUTED)),
            chip("F4", TEXT, SURFACE),
            Span::styled(" context   ", Style::default().fg(MUTED)),
            chip("Tab", TEXT, SURFACE),
            Span::styled(" next   ", Style::default().fg(MUTED)),
            chip("Ctrl+W", TEXT, SURFACE),
            Span::styled(
                format!(" write   {}", error_hint),
                Style::default().fg(MUTED),
            ),
        ]))
        .style(Style::default().bg(BG)),
        layout[3],
    );
    if let Some(overlay) = &app.overlay {
        draw_overlay(frame, overlay, app);
    }
}
fn draw_header(frame: &mut ratatui::Frame, area: Rect, app: &App) {
    let model = if app.model().is_empty() {
        "default"
    } else {
        app.model()
    };
    let auth_color = if app.auth.contains("ready") {
        TEAL
    } else {
        RED
    };
    let mode = if app.write {
        chip("WRITE: ARMED", BG, PEACH)
    } else {
        chip("READ ONLY", TEAL, SURFACE)
    };
    frame.render_widget(
        Paragraph::new(Line::from(vec![
            Span::styled(
                " ✦ nano ",
                Style::default()
                    .fg(BG)
                    .bg(LAVENDER)
                    .add_modifier(Modifier::BOLD),
            ),
            chip(app.profile().label, LAVENDER, SURFACE),
            chip(model, TEXT, SURFACE),
            chip(format!("● {}", app.auth), auth_color, SURFACE),
            mode,
        ]))
        .style(Style::default().bg(OVERLAY)),
        area,
    );
}
fn draw_conversation(frame: &mut ratatui::Frame, area: Rect, app: &App) {
    let mut lines: Vec<Line> = app
        .messages
        .iter()
        .rev()
        .skip(app.scroll)
        .take(30)
        .rev()
        .flat_map(|message| {
            let (accent, label) = if message.error {
                (RED, "ERROR")
            } else if message.who == "you" {
                (TEAL, "YOU")
            } else if message.who == "nano" {
                (LAVENDER, "NANO")
            } else {
                (PEACH, message.who.as_str())
            };
            vec![
                Line::from(vec![
                    Span::styled("▎ ", Style::default().fg(accent)),
                    chip(label.to_string(), accent, SURFACE),
                ]),
                Line::from(Span::styled(
                    message.text.clone(),
                    Style::default().fg(TEXT),
                )),
                Line::raw(""),
            ]
        })
        .collect();
    if app.pending.is_some() {
        lines.extend([
            Line::raw(""),
            Line::from(vec![
                Span::styled("● ", Style::default().fg(LAVENDER)),
                Span::styled(
                    format!("{} is thinking…", app.profile().label),
                    Style::default().fg(MUTED).add_modifier(Modifier::ITALIC),
                ),
            ]),
        ]);
    }
    frame.render_widget(
        Paragraph::new(lines)
            .wrap(Wrap { trim: false })
            .block(panel(" CONVERSATION ")),
        area,
    );
}
fn draw_sidebar(frame: &mut ratatui::Frame, area: Rect, app: &App) {
    let model = if app.model().is_empty() {
        "Vendor default"
    } else {
        app.model()
    };
    let access = if app.write {
        "ARMED — confirmation required"
    } else {
        "Read-only"
    };
    let lines = vec![
        Line::from(Span::styled(
            "SESSION",
            Style::default().fg(LAVENDER).add_modifier(Modifier::BOLD),
        )),
        Line::raw(""),
        Line::from(Span::styled("BACKEND", Style::default().fg(MUTED))),
        Line::from(Span::styled(
            app.profile().label,
            Style::default().fg(TEXT).add_modifier(Modifier::BOLD),
        )),
        Line::raw(""),
        Line::from(Span::styled("MODEL", Style::default().fg(MUTED))),
        Line::from(Span::styled(model, Style::default().fg(TEXT))),
        Line::raw(""),
        Line::from(Span::styled("AUTH", Style::default().fg(MUTED))),
        Line::from(Span::styled(
            format!("● {}", app.auth),
            Style::default().fg(if app.auth.contains("ready") {
                TEAL
            } else {
                RED
            }),
        )),
        Line::raw(""),
        Line::from(Span::styled("ACCESS", Style::default().fg(MUTED))),
        Line::from(Span::styled(
            access,
            Style::default().fg(if app.write { PEACH } else { TEAL }),
        )),
        Line::raw(""),
        Line::from(Span::styled("CONTEXT", Style::default().fg(MUTED))),
        Line::from(Span::styled(
            format!(
                "{} · {} cites",
                if app.context_enabled {
                    "LOCAL ON"
                } else {
                    "OFF"
                },
                app.citations.len()
            ),
            Style::default().fg(if app.context_enabled { LAVENDER } else { MUTED }),
        )),
        Line::raw(""),
        Line::from(Span::styled(
            "COMMANDS",
            Style::default().fg(LAVENDER).add_modifier(Modifier::BOLD),
        )),
        Line::from(Span::styled(
            "/help     command list",
            Style::default().fg(MUTED),
        )),
        Line::from(Span::styled(
            "/status   refresh auth",
            Style::default().fg(MUTED),
        )),
        Line::from(Span::styled(
            "/new      clear chat",
            Style::default().fg(MUTED),
        )),
    ];
    frame.render_widget(
        Paragraph::new(lines)
            .wrap(Wrap { trim: true })
            .block(panel(" INSPECTOR ")),
        area,
    );
}
fn draw_composer(frame: &mut ratatui::Frame, area: Rect, app: &App) {
    let title = if app.pending.is_some() {
        " WORKING "
    } else {
        " ASK NANO "
    };
    let border = if app.pending.is_some() {
        MUTED
    } else {
        LAVENDER
    };
    let block = Block::default()
        .title(Span::styled(
            title,
            Style::default().fg(border).add_modifier(Modifier::BOLD),
        ))
        .borders(Borders::ALL)
        .border_type(BorderType::Rounded)
        .border_style(Style::default().fg(border))
        .style(Style::default().bg(OVERLAY))
        .padding(Padding::new(1, 1, 0, 0));
    let inner = block.inner(area);
    let input = if app.pending.is_some() {
        format!(
            "{} is working. You can still browse the transcript.",
            app.profile().label
        )
    } else if app.input.is_empty() {
        "Type a request, or start with / for a command…".into()
    } else {
        app.input.clone()
    };
    let style = if app.input.is_empty() || app.pending.is_some() {
        Style::default().fg(MUTED)
    } else {
        Style::default().fg(TEXT)
    };
    frame.render_widget(
        Paragraph::new(input)
            .style(style)
            .wrap(Wrap { trim: false })
            .block(block),
        area,
    );
    if app.pending.is_none() && app.overlay.is_none() && inner.width > 0 && inner.height > 0 {
        let before: String = app.input.chars().take(app.cursor).collect();
        let width = UnicodeWidthStr::width(before.as_str()) as u16;
        let columns = inner.width.max(1);
        frame.set_cursor_position((
            inner.x + width % columns,
            inner.y + (width / columns).min(inner.height - 1),
        ));
    }
}
fn draw_overlay(frame: &mut ratatui::Frame, overlay: &Overlay, app: &App) {
    let area = centered(68, 62, frame.area());
    let (title, lines) = match overlay {
        Overlay::Provider(selected) => (
            " PROVIDERS ",
            PROFILES
                .iter()
                .enumerate()
                .map(|(index, item)| {
                    let selected_style = Style::default()
                        .fg(BG)
                        .bg(LAVENDER)
                        .add_modifier(Modifier::BOLD);
                    let normal = Style::default().fg(TEXT);
                    Line::from(if index == *selected {
                        vec![Span::styled(
                            format!(" › {}  {}", item.label, app.auths[index]),
                            selected_style,
                        )]
                    } else {
                        vec![Span::styled(format!("   {}", item.label), normal)]
                    })
                })
                .chain(std::iter::once(Line::raw("")))
                .chain(std::iter::once(Line::from(Span::styled(
                    "↑↓ navigate  ·  Enter select  ·  Esc cancel",
                    Style::default().fg(MUTED),
                ))))
                .collect(),
        ),
        Overlay::Model(selected) => (
            " MODELS ",
            app.profile()
                .models
                .iter()
                .enumerate()
                .map(|(index, model)| {
                    let label = if model.is_empty() {
                        "Vendor default"
                    } else {
                        model
                    };
                    Line::from(if index == *selected {
                        vec![Span::styled(
                            format!(" › {label}"),
                            Style::default()
                                .fg(BG)
                                .bg(LAVENDER)
                                .add_modifier(Modifier::BOLD),
                        )]
                    } else {
                        vec![Span::styled(
                            format!("   {label}"),
                            Style::default().fg(TEXT),
                        )]
                    })
                })
                .chain(std::iter::once(Line::raw("")))
                .chain(std::iter::once(Line::from(Span::styled(
                    "↑↓ navigate  ·  Enter select  ·  Esc cancel",
                    Style::default().fg(MUTED),
                ))))
                .collect(),
        ),
        Overlay::Confirm(request) => (
            " CONFIRM REQUEST ",
            vec![
                Line::from(Span::styled(
                    if request.write {
                        "Codex will receive workspace-write access."
                    } else {
                        "Your selected local source excerpts will leave this machine."
                    },
                    Style::default().fg(PEACH).add_modifier(Modifier::BOLD),
                )),
                Line::from(Span::styled(
                    if request.citations > 0 {
                        format!(
                            "{} cited excerpts will be attached to {}.",
                            request.citations, request.provider
                        )
                    } else {
                        "No source excerpts are attached.".into()
                    },
                    Style::default().fg(TEXT),
                )),
                Line::raw(""),
                Line::from(Span::styled(
                    "Enter / y confirms  ·  Esc / n keeps excerpts local",
                    Style::default().fg(YELLOW),
                )),
            ],
        ),
        Overlay::Detail(detail) => (
            " DETAIL ",
            detail
                .lines()
                .take(16)
                .map(|line| Line::from(Span::styled(line.to_owned(), Style::default().fg(TEXT))))
                .chain(std::iter::once(Line::raw("")))
                .chain(std::iter::once(Line::from(Span::styled(
                    "Enter / Esc / q closes",
                    Style::default().fg(MUTED),
                ))))
                .collect(),
        ),
    };
    frame.render_widget(Clear, area);
    frame.render_widget(
        Paragraph::new(lines)
            .alignment(Alignment::Left)
            .wrap(Wrap { trim: false })
            .block(
                Block::default()
                    .title(Span::styled(
                        title,
                        Style::default().fg(LAVENDER).add_modifier(Modifier::BOLD),
                    ))
                    .borders(Borders::ALL)
                    .border_type(BorderType::Rounded)
                    .border_style(Style::default().fg(LAVENDER))
                    .style(Style::default().bg(OVERLAY))
                    .padding(Padding::new(1, 1, 1, 1)),
            ),
        area,
    );
}
fn centered(width: u16, height: u16, area: Rect) -> Rect {
    let width = width.min(96);
    let height = height.min(90);
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
