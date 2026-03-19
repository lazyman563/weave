# ⚡ weave

Cliente HTTP pesado para terminal. Salva requests, encadeia flows, renderiza páginas com overlay de inspeção e corrige elementos com IA.

## 📦 Instalação
```bash
go install github.com/lazyman563/weave@latest
```

> Requer Go instalado. No Termux: `pkg install golang -y`

## 🚀 Uso rápido
```bash
# Adicionar requisição
weave request add --name "login" --url https://api.site.com/auth --method POST \
  --header "Content-Type:application/json" \
  --body '{"email":"user@site.com","password":"senha"}' \
  --expect-status 200

# Executar
weave request run "login"

# Ver lista
weave request list
```

## 🔗 Flows encadeados
```bash
weave flow add --name "completo" --chain "login,perfil,logout" --delay 300
weave flow run "completo"
```

## 🌐 Proxy Visual
```bash
weave serve --target https://site.com
```

Acesse `http://127.0.0.1:8080` no navegador — o site aparece com overlay do weave por cima. Clique em qualquer elemento, mande uma instrução e a IA corrige em tempo real.

## 🤖 Fix com IA
```bash
weave fix \
  --html '<button id="btn">Clique</button>' \
  --instruction "muda o texto pra Enviar e deixa azul"
```

Usa Pollinations AI — sem API key, sem conta.

## 📋 Histórico
```bash
weave history
weave history --last 50
weave history --format table
```

## ⚙️ Config (.weaverc)
```bash
weave config init    # cria exemplo
weave config import  # importa pro banco
weave config show    # mostra configuração
```

## 📚 Todos os comandos
```
weave request add/run/list/show/delete
weave flow add/run/list
weave history
weave serve
weave fix
weave config init/import/show/path
```
