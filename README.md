# ⚡ weave

> Cliente HTTP pesado para terminal. Salva requests, encadeia flows, renderiza páginas com overlay de inspeção e corrige elementos com IA.

---

## 📦 Instalação no Termux

### 1. Dependências

```bash
pkg update && pkg upgrade -y
pkg install golang git sqlite libsqlite clang make -y
```

### 2. Clone e compile

```bash
cd ~
git clone https://github.com/SEU_USUARIO/weave
cd weave
go mod tidy
go build -o weave .
```

### 3. Instala globalmente

```bash
mkdir -p $PREFIX/bin
cp weave $PREFIX/bin/weave
chmod +x $PREFIX/bin/weave
```

### 4. (Opcional) API da IA

```bash
export ANTHROPIC_API_KEY=sua_chave_aqui
# Adicione no ~/.bashrc para persistir:
echo 'export ANTHROPIC_API_KEY=sua_chave' >> ~/.bashrc
```

---

## 🚀 Uso rápido

### Adicionar uma requisição

```bash
weave request add \
  --name "login" \
  --url https://api.exemplo.com/auth \
  --method POST \
  --header "Content-Type:application/json" \
  --body '{"email":"user@exemplo.com","password":"senha"}' \
  --expect-button "Entrar" \
  --expect-status 200
```

### Executar

```bash
weave request run "login"
weave request run "login" --verbose
```

### Ver lista

```bash
weave request list
```

---

## 🔗 Flows encadeados

```bash
# Criar flow
weave flow add --name "completo" --chain "login,perfil,logout" --delay 300

# Executar
weave flow run "completo"

# Ou inline sem salvar
weave flow run --chain "login,perfil" --delay 500
```

### Passar output entre steps

No body de um request, use `{{previous}}` para injetar a resposta do step anterior:

```bash
weave request add --name "usar-token" \
  --url https://api.site.com/me \
  --method GET \
  --header "Authorization:Bearer {{previous}}"
```

---

## 🌐 Proxy Visual (o modo navegador)

```bash
weave serve --target https://site.com
```

Abre um servidor local. Acesse no navegador do celular ou de outro dispositivo na mesma rede:

```
http://192.168.x.x:8080
```

A página do site aparece com o **overlay do weave** por cima:
- 🔍 **Inspect mode** — clique em qualquer elemento para selecioná-lo
- 🔧 **Fix Panel** — escreva uma instrução e manda pra IA corrigir

---

## 🤖 Fix com IA

### Via CLI

```bash
weave fix \
  --element "#btn-login" \
  --html '<button id="btn-login">Login</button>' \
  --instruction "esse botão deve fazer POST em /auth/v2 com fetch"
```

### Via painel visual

1. `weave serve --target https://site.com`
2. Abra no navegador
3. Clique em **Inspecionar** → clique num elemento
4. Escreva a instrução → **Enviar para IA ⚡**

---

## 📋 Histórico

```bash
weave history
weave history --last 50
weave history --format table
```

---

## ⚙️ Config (.weaverc)

```bash
# Gera arquivo de exemplo
weave config init

# Edite
nano ~/.weaverc

# Importa pro banco
weave config import

# Mostra configuração atual
weave config show
```

### Exemplo de `.weaverc`

```yaml
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

  perfil:
    url: https://api.exemplo.com/me
    method: GET
    headers:
      Authorization: "Bearer TOKEN"
    expect:
      status: 200

flows:
  login-completo:
    chain:
      - login
      - perfil
    delay: 500
```

---

## 📚 Todos os comandos

```
weave request add       Adiciona/atualiza uma requisição
weave request run       Executa uma requisição
weave request list      Lista requisições salvas
weave request show      Mostra detalhes
weave request delete    Remove uma requisição

weave flow add          Cria um flow
weave flow run          Executa um flow
weave flow list         Lista flows

weave history           Histórico de execuções
weave serve             Proxy visual com overlay
weave fix               Corrige elementos com IA
weave config init       Cria .weaverc de exemplo
weave config import     Importa do .weaverc pro banco
weave config show       Mostra configuração
weave config path       Caminho do .weaverc
```

---

## 🗂️ Estrutura do projeto

```
weave/
├── main.go
├── go.mod
├── cmd/
│   ├── root.go       # CLI principal
│   ├── request.go    # weave request
│   ├── flow.go       # weave flow
│   └── other.go      # history, serve, fix, config
└── internal/
    ├── storage/      # SQLite
    ├── proxy/        # HTTP runner + servidor proxy
    ├── ai/           # Integração com Claude API
    └── config/       # .weaverc loader
```

---

## 📍 Dados salvos

Todos os dados ficam em `~/.weave/weave.db` (SQLite).  
Config em `~/.weaverc`.

---

Feito pra rodar pesado no Termux. ⚡
