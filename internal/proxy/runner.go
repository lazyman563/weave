package proxy

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lazyman563/weave/internal/storage"
)

type RunResult struct {
	StatusCode  int
	Body        string
	Headers     map[string]string
	Duration    time.Duration
	Error       string
	Violations  []Violation
}

type Violation struct {
	Type    string // "button", "field", "status"
	Missing string
	Message string
}

func RunRequest(r *storage.Request) (*RunResult, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	bodyReader := strings.NewReader(r.Body)
	req, err := http.NewRequest(r.Method, r.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("requisição inválida: %w", err)
	}

	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	if _, ok := r.Headers["Content-Type"]; !ok && r.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "github.com/lazyman563/weave/1.0 (terminal HTTP client)")

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	result := &RunResult{Duration: duration}

	if err != nil {
		result.Error = err.Error()
		return result, nil
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	result.StatusCode = resp.StatusCode
	result.Body = string(bodyBytes)
	result.Headers = make(map[string]string)
	for k, v := range resp.Header {
		result.Headers[k] = strings.Join(v, ", ")
	}

	// Check expectations
	result.Violations = checkExpectations(r.Expect, result)

	return result, nil
}

func checkExpectations(expect storage.Expectation, result *RunResult) []Violation {
	var violations []Violation

	if expect.Status != 0 && result.StatusCode != expect.Status {
		violations = append(violations, Violation{
			Type:    "status",
			Missing: fmt.Sprintf("%d", expect.Status),
			Message: fmt.Sprintf("esperado status %d, recebido %d", expect.Status, result.StatusCode),
		})
	}

	bodyLower := strings.ToLower(result.Body)
	for _, btn := range expect.Buttons {
		if !strings.Contains(bodyLower, strings.ToLower(btn)) {
			violations = append(violations, Violation{
				Type:    "button",
				Missing: btn,
				Message: fmt.Sprintf("botão '%s' não encontrado na resposta", btn),
			})
		}
	}

	for _, field := range expect.Fields {
		if !strings.Contains(bodyLower, strings.ToLower(field)) {
			violations = append(violations, Violation{
				Type:    "field",
				Missing: field,
				Message: fmt.Sprintf("campo '%s' não encontrado na resposta", field),
			})
		}
	}

	return violations
}
