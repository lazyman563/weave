package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/lazyman563/weave/internal/ai"
	"github.com/lazyman563/weave/internal/config"
	"github.com/lazyman563/weave/internal/proxy"
	"github.com/lazyman563/weave/internal/storage"
)

// ─── HISTORY ────────────────────────────────────────────────────────────────

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Ver histórico de requisições executadas",
	Long: `Exibe o histórico de todas as requisições executadas.

EXEMPLOS:
  weave history
  weave history --last 50
  weave history --format table`,
	RunE: func(cmd *cobra.Command, args []string) error {
		last, _ := cmd.Flags().GetInt("last")
		format, _ := cmd.Flags().GetString("format")

		db, err := storage.Open()
		if err != nil {
			return err
		}
		defer db.Close()

		entries, err := db.GetHistory(last)
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			fmt.Println(styleInfo.Render("Nenhuma requisição no histórico ainda."))
			return nil
		}

		if format == "table" {
			fmt.Printf("\n%-5s %-20s %-8s %-6s %-8s %s\n",
				styleHighlight.Render("ID"),
				styleHighlight.Render("NOME"),
				styleHighlight.Render("STATUS"),
				styleHighlight.Render("MS"),
				styleHighlight.Render("HORA"),
				styleHighlight.Render("URL"),
			)
			fmt.Println(styleInfo.Render(strings.Repeat("─", 80)))
			for _, h := range entries {
				statusStyle := styleSuccess
				if h.StatusCode >= 400 {
					statusStyle = styleError
				}
				fmt.Printf("%-5d %-20s %-8s %-6d %-8s %s\n",
					h.ID,
					styleHighlight.Render(truncStr(h.Name, 18)),
					statusStyle.Render(fmt.Sprintf("%d", h.StatusCode)),
					h.Duration,
					h.Timestamp.Format("15:04:05"),
					truncStr(h.URL, 40),
				)
			}
			fmt.Println()
		} else {
			for _, h := range entries {
				statusStyle := styleSuccess
				if h.StatusCode >= 400 {
					statusStyle = styleError
				}
				errStr := ""
				if h.Error != "" {
					errStr = " " + styleError.Render("ERR: "+h.Error)
				}
				fmt.Printf("%s %s %s  %dms  %s%s\n",
					styleInfo.Render(h.Timestamp.Format("15:04:05")),
					statusStyle.Render(fmt.Sprintf("HTTP %d", h.StatusCode)),
					styleHighlight.Render(h.Name),
					h.Duration,
					styleInfo.Render(h.URL),
					errStr,
				)
			}
		}
		return nil
	},
}

// ─── SERVE ──────────────────────────────────────────────────────────────────

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Inicia o proxy visual com overlay do weave",
	Long: `Inicia um servidor proxy reverso que renderiza o site alvo
com a interface de inspeção do weave injetada.

EXEMPLOS:
  weave serve --target https://site.com
  weave serve --target https://site.com --port 9090`,
	RunE: func(cmd *cobra.Command, args []string) error {
		target, _ := cmd.Flags().GetString("target")
		port, _ := cmd.Flags().GetInt("port")

		if target == "" {
			return fmt.Errorf("--target é obrigatório. Ex: weave serve --target https://site.com")
		}

		srv := proxy.NewProxyServer(target, port)

		fmt.Printf("\n%s\n\n", styleBrand.Render("⚡ WEAVE SERVE"))
		fmt.Printf("  %s %s\n", styleInfo.Render("Alvo:"), styleHighlight.Render(target))
		fmt.Printf("  %s %s\n", styleInfo.Render("Acesse no navegador:"), styleHighlight.Render(srv.Address()))
		fmt.Printf("  %s %s\n\n", styleInfo.Render("Na mesma rede Wi-Fi:"), styleHighlight.Render(srv.WeaveAddress()))
		fmt.Println(styleInfo.Render("  Ctrl+C para parar\n"))

		return srv.Start()
	},
}

// ─── FIX ────────────────────────────────────────────────────────────────────

var fixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Corrige elementos de página via IA",
	Long: `Usa IA para corrigir elementos HTML de uma página.
Pode ser usado via CLI ou pelo painel visual do weave serve.

EXEMPLOS:
  weave fix --element "#submit-btn" --html '<button id="submit-btn">Enviar</button>' \
    --instruction "esse botão deve fazer POST em /auth/v2"
  weave fix --serve   (inicia endpoint de fix para o painel visual)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		element, _ := cmd.Flags().GetString("element")
		instruction, _ := cmd.Flags().GetString("instruction")
		html, _ := cmd.Flags().GetString("html")
		serve, _ := cmd.Flags().GetBool("serve")

		if serve {
			return startFixServer()
		}

		if instruction == "" || html == "" {
			return fmt.Errorf("--instruction e --html são obrigatórios")
		}

		fmt.Printf("\n%s Consultando IA...\n", styleInfo.Render("⚡"))
		_ = element

		result, err := ai.Fix(&ai.FixRequest{
			Selector:    element,
			Instruction: instruction,
			HTML:        html,
		})
		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Println(styleSuccess.Render("✓ CORREÇÃO:"))
		fmt.Println(styleInfo.Render(result.Explanation))
		if result.FixedHTML != "" {
			fmt.Println()
			fmt.Println(styleHighlight.Render("HTML CORRIGIDO:"))
			fmt.Println(result.FixedHTML)
		}
		return nil
	},
}

func startFixServer() error {
	fmt.Printf("\n%s Fix server em http://localhost:7777/__weave__/fix\n", styleBrand.Render("⚡"))

	http.HandleFunc("/__weave__/fix", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", 405)
			return
		}
		var req struct {
			Selector    string `json:"selector"`
			Instruction string `json:"instruction"`
			HTML        string `json:"html"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		result, err := ai.Fix(&ai.FixRequest{
			Selector:    req.Selector,
			Instruction: req.Instruction,
			HTML:        req.HTML,
		})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"result":     result.Explanation,
			"fixed_html": result.FixedHTML,
		})
	})

	return http.ListenAndServe(":7777", nil)
}

// ─── CONFIG ─────────────────────────────────────────────────────────────────

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Gerenciar configuração do weave (.weaverc)",
	Long: `Gerencia o arquivo de configuração ~/.weaverc.

SUBCOMANDOS:
  init    Cria um .weaverc de exemplo
  show    Mostra a configuração atual
  path    Mostra o caminho do arquivo
  import  Importa requests/flows do .weaverc para o banco

EXEMPLOS:
  weave config init
  weave config show
  weave config import`,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Cria um .weaverc de exemplo",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.CreateExample(); err != nil {
			return err
		}
		fmt.Printf("%s .weaverc criado em %s\n", styleSuccess.Render("✓"), styleHighlight.Render(config.Path()))
		fmt.Println(styleInfo.Render("  Edite o arquivo e depois rode: weave config import"))
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Mostra a configuração atual",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(config.Path())
		if os.IsNotExist(err) {
			fmt.Println(styleInfo.Render("Nenhum .weaverc encontrado. Execute: weave config init"))
			return nil
		}
		fmt.Printf("\n%s\n\n%s\n", styleHighlight.Render(config.Path()), string(data))
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Mostra o caminho do .weaverc",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.Path())
	},
}

var configImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Importa requests e flows do .weaverc para o banco",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := config.Load()
		if err != nil {
			return err
		}

		db, err := storage.Open()
		if err != nil {
			return err
		}
		defer db.Close()

		imported := 0
		for name, rc := range rc.Requests {
			r := &storage.Request{
				Name:    name,
				URL:     rc.URL,
				Method:  rc.Method,
				Headers: rc.Headers,
				Body:    rc.Body,
				Expect: storage.Expectation{
					Buttons: rc.Expect.Buttons,
					Fields:  rc.Expect.Fields,
					Status:  rc.Expect.Status,
				},
			}
			if r.Method == "" {
				r.Method = "GET"
			}
			if err := db.SaveRequest(r); err == nil {
				fmt.Printf("  %s request: %s\n", styleSuccess.Render("✓"), styleHighlight.Render(name))
				imported++
			}
		}

		for name, fc := range rc.Flows {
			if err := db.SaveFlow(&storage.Flow{Name: name, Chain: fc.Chain, Delay: fc.Delay}); err == nil {
				fmt.Printf("  %s flow: %s\n", styleSuccess.Render("✓"), styleHighlight.Render(name))
				imported++
			}
		}

		fmt.Printf("\n%s %d itens importados.\n", styleSuccess.Render("✓"), imported)
		return nil
	},
}

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func init() {
	historyCmd.Flags().Int("last", 20, "Número de entradas a exibir")
	historyCmd.Flags().String("format", "default", "Formato: default, table")

	serveCmd.Flags().String("target", "", "URL do site alvo (obrigatório)")
	serveCmd.Flags().Int("port", 8080, "Porta local do servidor")

	fixCmd.Flags().String("element", "", "Seletor CSS do elemento")
	fixCmd.Flags().String("instruction", "", "O que deve ser corrigido")
	fixCmd.Flags().String("html", "", "HTML do elemento a corrigir")
	fixCmd.Flags().Bool("serve", false, "Inicia endpoint HTTP de fix na porta 7777")

	configCmd.AddCommand(configInitCmd, configShowCmd, configPathCmd, configImportCmd)

	_ = time.Now() // keep import
}
