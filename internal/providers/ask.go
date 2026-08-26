package providers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 120 * time.Second}

func Ask(provider, prompt, model string, write bool) (string, error) {
	// Low-level transport. App code must call harness.Send / harness.Run instead.
	profile, ok := Find(provider)
	if !ok {
		return "", fmt.Errorf("provider must be codex, prime, openai, anthropic, or pi")
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
	case "prime":
		return askPrime(prompt, model, write)
	case "pi":
		return askCommand("pi", append([]string{"--print"}, optionalModel(model)...), prompt)
	}
	return "", errors.New("unreachable")
}

func askPrime(prompt, model string, write bool) (string, error) {
	if _, err := exec.LookPath("prime-agent"); err != nil {
		return "", errors.New("prime-agent not installed; curl -fsSL https://app.primeintellect.ai/prime-agent/install.sh | sh")
	}
	args := []string{"--print", "--no-session"}
	if !write {
		// Match nanoharness read-only default: answer without mutating the tree.
		args = append(args, "--no-tools")
	}
	args = append(args, optionalModel(model)...)
	text, err := askCommand("prime-agent", args, prompt)
	if err != nil {
		detail := err.Error()
		if strings.Contains(detail, "auth") || strings.Contains(detail, "login") || strings.Contains(detail, "401") {
			return "", fmt.Errorf("%w\n\nRun `nanoharness login prime`, then `/login` inside prime-agent.", err)
		}
		return "", err
	}
	return text, nil
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
	request, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if response.StatusCode/100 != 2 {
		return "", fmt.Errorf("OpenAI returned %s: %s", response.Status, truncate(string(data), 800))
	}
	text := openAIText(data)
	if text == "" {
		return "", errors.New("OpenAI returned no text")
	}
	return text, nil
}

func openAIText(data []byte) string {
	var result struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if json.Unmarshal(data, &result) != nil {
		return ""
	}
	if strings.TrimSpace(result.OutputText) != "" {
		return result.OutputText
	}
	var out strings.Builder
	for _, item := range result.Output {
		for _, content := range item.Content {
			out.WriteString(content.Text)
		}
	}
	return out.String()
}

func askAnthropic(prompt, model string) (string, error) {
	token := key("ANTHROPIC_API_KEY")
	if token == "" {
		return "", errors.New("missing ANTHROPIC_API_KEY; run `nanoharness login anthropic`")
	}
	body, _ := json.Marshal(map[string]any{"model": model, "max_tokens": 4096, "messages": []map[string]string{{"role": "user", "content": prompt}}})
	request, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("x-api-key", token)
	request.Header.Set("anthropic-version", "2023-06-01")
	request.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if response.StatusCode/100 != 2 {
		return "", fmt.Errorf("Anthropic returned %s: %s", response.Status, truncate(string(data), 800))
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
