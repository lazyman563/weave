package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type FixRequest struct {
	Selector    string
	Instruction string
	HTML        string
}

type FixResult struct {
	FixedHTML   string
	Explanation string
}

type pollinationsMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type pollinationsReq struct {
	Model    string            `json:"model"`
	Messages []pollinationsMsg `json:"messages"`
	Private  bool              `json:"private"`
}

func callPollinations(prompt string) (string, error) {
	payload := pollinationsReq{
		Model: "openai",
		Messages: []pollinationsMsg{
			{Role: "system", Content: "Você é um especialista em HTML/CSS/JavaScript. Responda sempre em português."},
			{Role: "user", Content: prompt},
		},
		Private: true,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://text.pollinations.ai/openai", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("erro ao criar requisição: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("erro ao chamar Pollinations: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBytes, &result); err != nil {
		text := strings.TrimSpace(string(respBytes))
		if text != "" {
			return text, nil
		}
		return "", fmt.Errorf("resposta inválida da API")
	}

	if result.Error != nil {
		return "", fmt.Errorf("erro da API: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("resposta vazia da API")
	}

	return result.Choices[0].Message.Content, nil
}

func Fix(r *FixRequest) (*FixResult, error) {
	prompt := fmt.Sprintf(`Você é um especialista em HTML/CSS/JavaScript.

O usuário selecionou este elemento HTML:
<elemento>
%s
</elemento>

Instrução do usuário:
"%s"

Retorne APENAS um JSON válido neste formato exato (sem markdown, sem código extra):
{"fixed_html": "HTML_CORRIGIDO_AQUI", "explanation": "O que foi alterado em português"}

O HTML corrigido deve ser funcional e aplicar exatamente o que o usuário pediu.`, r.HTML, r.Instruction)

	text, err := callPollinations(prompt)
	if err != nil {
		return nil, err
	}

	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var parsed struct {
		FixedHTML   string `json:"fixed_html"`
		Explanation string `json:"explanation"`
	}

	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return &FixResult{Explanation: text}, nil
	}

	return &FixResult{
		FixedHTML:   parsed.FixedHTML,
		Explanation: parsed.Explanation,
	}, nil
}

func AnalyzePage(html string, expectations []string) (string, error) {
	expectStr := ""
	for _, e := range expectations {
		expectStr += "- " + e + "\n"
	}

	prompt := fmt.Sprintf(`Analise este HTML e diga se os seguintes elementos estão presentes:
%s

HTML:
%s

Responda em português, de forma direta e técnica. Liste o que foi encontrado e o que está faltando.`,
		expectStr, truncate(html, 3000))

	return callPollinations(prompt)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...[truncado]"
}

