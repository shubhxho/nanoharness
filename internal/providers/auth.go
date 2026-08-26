package providers

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

func configPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "nanoharness", "credentials")
}
func credentials() map[string]string {
	file, err := os.Open(configPath())
	if err != nil {
		return map[string]string{}
	}
	defer file.Close()
	out := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if key, value, ok := strings.Cut(scanner.Text(), "="); ok {
			out[key] = value
		}
	}
	return out
}
func key(name string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return credentials()[name]
}
func SaveKey(name string) error {
	fmt.Fprint(os.Stderr, "API key: ")
	value, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return err
	}
	if len(value) == 0 {
		return errors.New("empty API key")
	}
	values := credentials()
	values[name] = string(value)
	if err := os.MkdirAll(filepath.Dir(configPath()), 0700); err != nil {
		return err
	}
	var body strings.Builder
	for key, value := range values {
		fmt.Fprintf(&body, "%s=%s\n", key, value)
	}
	return os.WriteFile(configPath(), []byte(body.String()), 0600)
}

func Login(kind string, apiKey bool) error {
	switch kind {
	case "openai":
		return SaveKey("OPENAI_API_KEY")
	case "anthropic":
		return SaveKey("ANTHROPIC_API_KEY")
	case "claude":
		return vendor("claude")
	case "codex":
		if !apiKey {
			return vendor("codex", "login")
		}
		fmt.Fprint(os.Stderr, "API key: ")
		value, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}
		command := exec.Command("codex", "login", "--with-api-key")
		command.Stdin = bytes.NewReader(value)
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		return command.Run()
	case "prime", "prime-agent":
		if _, err := exec.LookPath("prime-agent"); err != nil {
			return fmt.Errorf("prime-agent not installed; curl -fsSL https://app.primeintellect.ai/prime-agent/install.sh | sh")
		}
		fmt.Fprintln(os.Stderr, "Starting prime-agent — run /login for Prime Intellect / provider auth, then exit the session.")
		return vendor("prime-agent")
	default:
		return fmt.Errorf("login provider must be codex, prime, openai, anthropic, or claude")
	}
}
func vendor(name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	return command.Run()
}
func AuthStatus(provider string) string {
	switch provider {
	case "openai":
		if key("OPENAI_API_KEY") != "" {
			return "API key configured"
		}
		return "missing OPENAI_API_KEY"
	case "anthropic":
		if key("ANTHROPIC_API_KEY") != "" {
			return "API key configured"
		}
		return "missing ANTHROPIC_API_KEY"
	case "codex":
		if exec.Command("codex", "login", "status").Run() == nil {
			return "native login ready"
		}
		return "login needed"
	case "prime":
		if _, err := exec.LookPath("prime-agent"); err != nil {
			return "CLI unavailable"
		}
		if primeAuthReady() {
			return "prime-agent auth ready"
		}
		return "login needed (/login)"
	case "pi":
		if exec.Command("pi", "--version").Run() == nil {
			return "CLI ready"
		}
		return "CLI unavailable"
	}
	return "unknown provider"
}

func primeAuthReady() bool {
	home := os.Getenv("HOME")
	if home == "" {
		return false
	}
	path := filepath.Join(home, ".prime", "agent", "auth.json")
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return false
	}
	trimmed := strings.TrimSpace(string(data))
	return trimmed != "" && trimmed != "{}" && trimmed != "[]"
}
