package providers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

type Profile struct {
	ID, Label, Default string
	Models             []string
}

var Profiles = []Profile{
	{"codex", "Codex", "", []string{"", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.5"}},
	{"openai", "OpenAI", "gpt-5.6-terra", []string{"gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.5"}},
	{"anthropic", "Anthropic", "claude-sonnet-5", []string{"claude-sonnet-5", "claude-haiku-4-5-20251001", "claude-opus-5"}},
	{"pi", "pi", "", []string{"", "openai-codex/gpt-5.6-terra", "openai-codex/gpt-5.5", "anthropic/claude-sonnet-5"}},
}

func Find(id string) (Profile, bool) {
	for _, p := range Profiles {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

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
	default:
		return fmt.Errorf("login provider must be codex, openai, anthropic, or claude")
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
	case "pi":
		if exec.Command("pi", "--version").Run() == nil {
			return "CLI ready"
		}
		return "CLI unavailable"
	}
	return "unknown provider"
}

func Ask(provider, prompt, model string, write bool) (string, error) {
	profile, ok := Find(provider)
	if !ok {
		return "", fmt.Errorf("provider must be codex, openai, anthropic, or pi")
	}
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("prompt is empty")
	}
	if model == "" {
		model = profile.Default
	}
	switch provider {
	case "openai":
		return askOpenAI(prompt, model)
	case "anthropic":
		return askAnthropic(prompt, model)
	case "codex":
		return askCommand("codex", append([]string{"exec", "--skip-git-repo-check", "--sandbox", map[bool]string{true: "workspace-write", false: "read-only"}[write]}, optionalModel(model)...), prompt)
	case "pi":
		return askCommand("pi", append([]string{"--print"}, optionalModel(model)...), prompt)
	}
	return "", errors.New("unreachable")
}
func optionalModel(model string) []string {
	if model == "" {
		return nil
	}
	return []string{"--model", model}
}
func askCommand(name string, args []string, prompt string) (string, error) {
	args = append(args, "--", prompt)
	command := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if err != nil {
		detail := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		if strings.Contains(detail, "401 Unauthorized") {
			detail += "\n\nAuthentication was rejected. Run `nanoharness login codex`, then retry."
		}
		return "", fmt.Errorf("%s failed: %s", name, detail)
	}
	return strings.TrimSpace(stdout.String()), nil
}
func askOpenAI(prompt, model string) (string, error) {
	token := key("OPENAI_API_KEY")
	if token == "" {
		return "", errors.New("missing OPENAI_API_KEY; run `nanoharness login openai`")
	}
	body, _ := json.Marshal(map[string]any{"model": model, "input": prompt})
	request, _ := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	if response.StatusCode/100 != 2 {
		return "", fmt.Errorf("OpenAI returned %s: %s", response.Status, data)
	}
	var result struct {
		Output []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	var out strings.Builder
	for _, item := range result.Output {
		for _, content := range item.Content {
			out.WriteString(content.Text)
		}
	}
	if out.Len() == 0 {
		return "", errors.New("OpenAI returned no text")
	}
	return out.String(), nil
}
func askAnthropic(prompt, model string) (string, error) {
	token := key("ANTHROPIC_API_KEY")
	if token == "" {
		return "", errors.New("missing ANTHROPIC_API_KEY; run `nanoharness login anthropic`")
	}
	body, _ := json.Marshal(map[string]any{"model": model, "max_tokens": 4096, "messages": []map[string]string{{"role": "user", "content": prompt}}})
	request, _ := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	request.Header.Set("x-api-key", token)
	request.Header.Set("anthropic-version", "2023-06-01")
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	if response.StatusCode/100 != 2 {
		return "", fmt.Errorf("Anthropic returned %s: %s", response.Status, data)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	var out strings.Builder
	for _, item := range result.Content {
		out.WriteString(item.Text)
	}
	if out.Len() == 0 {
		return "", errors.New("Anthropic returned no text")
	}
	return out.String(), nil
}
