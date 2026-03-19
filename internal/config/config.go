package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type WeaveRC struct {
	Flows    map[string]FlowConfig    `yaml:"flows"`
	Requests map[string]RequestConfig `yaml:"requests"`
	Defaults DefaultsConfig           `yaml:"defaults"`
}

type FlowConfig struct {
	Chain []string `yaml:"chain"`
	Delay int      `yaml:"delay"`
}

type RequestConfig struct {
	URL     string            `yaml:"url"`
	Method  string            `yaml:"method"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
	Expect  ExpectConfig      `yaml:"expect"`
}

type ExpectConfig struct {
	Buttons []string `yaml:"buttons"`
	Fields  []string `yaml:"fields"`
	Status  int      `yaml:"status"`
}

type DefaultsConfig struct {
	Timeout int               `yaml:"timeout"`
	Headers map[string]string `yaml:"headers"`
}

func Load() (*WeaveRC, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".weaverc")

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &WeaveRC{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("erro ao ler .weaverc: %w", err)
	}

	var rc WeaveRC
	if err := yaml.Unmarshal(data, &rc); err != nil {
		return nil, fmt.Errorf("erro ao parsear .weaverc: %w", err)
	}
	return &rc, nil
}

func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".weaverc")
}

func CreateExample() error {
	example := `# ~/.weaverc — Configuração do weave
# Documentação: weave config --help

defaults:
  timeout: 30
  headers:
    User-Agent: "github.com/lazyman563/weave/1.0"

requests:
  login:
    url: https://api.exemplo.com/auth
    method: POST
    headers:
      Content-Type: application/json
    body: '{"email":"user@exemplo.com","password":"senha"}'
    expect:
      status: 200
      buttons:
        - "Entrar"
        - "Esqueci a senha"
      fields:
        - "token"
        - "user_id"

  get-profile:
    url: https://api.exemplo.com/me
    method: GET
    headers:
      Authorization: "Bearer TOKEN_AQUI"
    expect:
      status: 200

flows:
  login-completo:
    chain:
      - login
      - get-profile
    delay: 500
`
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".weaverc")
	return os.WriteFile(path, []byte(example), 0644)
}
