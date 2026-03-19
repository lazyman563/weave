package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/lazyman563/weave/internal/proxy"
	"github.com/lazyman563/weave/internal/storage"
)

var requestCmd = &cobra.Command{
	Use:   "request",
	Short: "Gerenciar e executar requisições HTTP",
	Long: `Gerenciar e executar requisições HTTP salvas.

SUBCOMANDOS:
  add     Adiciona ou atualiza uma requisição
  run     Executa uma requisição pelo nome
  list    Lista todas as requisições salvas
  show    Mostra detalhes de uma requisição
  delete  Remove uma requisição

EXEMPLOS:
  weave request add --name "login" --url https://api.site.com/auth --method POST --body '{"user":"x"}'
  weave request run "login"
  weave request run "login" --verbose
  weave request list
  weave request show "login"
  weave request delete "login"`,
}

var reqAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Adiciona ou atualiza uma requisição",
	Example: `  weave request add --name "login" --url https://api.site.com/auth --method POST \
    --header "Content-Type:application/json" --body '{"user":"x","pass":"y"}' \
    --expect-button "Entrar" --expect-status 200`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		url, _ := cmd.Flags().GetString("url")
		method, _ := cmd.Flags().GetString("method")
		body, _ := cmd.Flags().GetString("body")
		headers, _ := cmd.Flags().GetStringArray("header")
		expectButtons, _ := cmd.Flags().GetStringArray("expect-button")
		expectFields, _ := cmd.Flags().GetStringArray("expect-field")
		expectStatus, _ := cmd.Flags().GetInt("expect-status")

		if name == "" || url == "" {
			return fmt.Errorf("--name e --url são obrigatórios")
		}

		parsedHeaders := make(map[string]string)
		for _, h := range headers {
			parts := strings.SplitN(h, ":", 2)
			if len(parts) == 2 {
				parsedHeaders[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}

		db, err := storage.Open()
		if err != nil {
			return err
		}
		defer db.Close()

		r := &storage.Request{
			Name:    name,
			URL:     url,
			Method:  strings.ToUpper(method),
			Headers: parsedHeaders,
			Body:    body,
			Expect: storage.Expectation{
				Buttons: expectButtons,
				Fields:  expectFields,
				Status:  expectStatus,
			},
		}

		if err := db.SaveRequest(r); err != nil {
			return err
		}

		fmt.Println(styleSuccess.Render("✓") + " Requisição " + styleHighlight.Render(name) + " salva.")
		return nil
	},
}

var reqRunCmd = &cobra.Command{
	Use:     "run [nome]",
	Short:   "Executa uma requisição pelo nome",
	Args:    cobra.ExactArgs(1),
	Example: `  weave request run "login"\n  weave request run "login" --verbose`,
	RunE: func(cmd *cobra.Command, args []string) error {
		verbose, _ := cmd.Flags().GetBool("verbose")
		raw, _ := cmd.Flags().GetBool("raw")

		db, err := storage.Open()
		if err != nil {
			return err
		}
		defer db.Close()

		r, err := db.GetRequest(args[0])
		if err != nil {
			return err
		}

		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		fmt.Printf("\r%s Executando %s...", spinner[0], styleHighlight.Render(r.Name))

		result, err := proxy.RunRequest(r)
		if err != nil {
			return err
		}

		fmt.Print("\r\033[K") // clear spinner

		printResult(r, result, verbose, raw)

		// save to history
		headers, _ := json.Marshal(result.Headers)
		db.SaveHistory(&storage.HistoryEntry{
			RequestID:  r.ID,
			Name:       r.Name,
			URL:        r.URL,
			Method:     r.Method,
			StatusCode: result.StatusCode,
			Body:       result.Body,
			Headers:    string(headers),
			Duration:   result.Duration.Milliseconds(),
			Error:      result.Error,
		})

		// violations
		if len(result.Violations) > 0 {
			ip := proxy.GetLocalIP()
			fmt.Println()
			fmt.Println(styleError.Render("⚠ VIOLAÇÕES DETECTADAS:"))
			for _, v := range result.Violations {
				fmt.Printf("  %s %s\n", styleError.Render("✗"), v.Message)
			}
			fmt.Println()
			fmt.Printf("  %s\n", styleInfo.Render("Para inspecionar visualmente, acesse:"))
			fmt.Printf("  %s\n\n", styleHighlight.Render(fmt.Sprintf("http://%s:8080", ip)))
			fmt.Printf("  %s\n", styleInfo.Render("Ou execute: weave serve --target "+r.URL))
		}

		return nil
	},
}

func printResult(r *storage.Request, result *proxy.RunResult, verbose, raw bool) {
	if result.Error != "" {
		fmt.Println(styleError.Render("✗ ERRO: " + result.Error))
		return
	}

	statusStyle := styleSuccess
	if result.StatusCode >= 400 {
		statusStyle = styleError
	} else if result.StatusCode >= 300 {
		statusStyle = styleHighlight
	}

	statusLine := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#333")).
		Padding(0, 1).
		Render(
			statusStyle.Render(fmt.Sprintf("HTTP %d", result.StatusCode)) +
				styleInfo.Render(fmt.Sprintf("  %s", r.Method)) +
				"  " + styleInfo.Render(r.URL) +
				styleInfo.Render(fmt.Sprintf("  %dms", result.Duration.Milliseconds())),
		)
	fmt.Println(statusLine)

	if verbose {
		fmt.Println(styleInfo.Render("\nHEADERS:"))
		for k, v := range result.Headers {
			fmt.Printf("  %s: %s\n", styleHighlight.Render(k), v)
		}
	}

	fmt.Println(styleInfo.Render("\nBODY:"))
	if raw {
		fmt.Println(result.Body)
	} else {
		var pretty interface{}
		if json.Unmarshal([]byte(result.Body), &pretty) == nil {
			b, _ := json.MarshalIndent(pretty, "  ", "  ")
			fmt.Println("  " + colorizeJSON(string(b)))
		} else {
			body := result.Body
			if len(body) > 500 {
				body = body[:500] + "\n" + styleInfo.Render("... [use --raw para ver tudo]")
			}
			fmt.Println(body)
		}
	}
}

func colorizeJSON(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	for _, line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			key := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFB3")).Render(parts[0])
			out = append(out, key+":"+parts[1])
		} else {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

var reqListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista todas as requisições salvas",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := storage.Open()
		if err != nil {
			return err
		}
		defer db.Close()

		reqs, err := db.ListRequests()
		if err != nil {
			return err
		}

		if len(reqs) == 0 {
			fmt.Println(styleInfo.Render("Nenhuma requisição salva. Use: weave request add"))
			return nil
		}

		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700"))
		fmt.Printf("\n%-20s %-8s %-45s %s\n",
			header.Render("NOME"),
			header.Render("MÉTODO"),
			header.Render("URL"),
			header.Render("EXPECTS"),
		)
		fmt.Println(styleInfo.Render(strings.Repeat("─", 90)))

		for _, r := range reqs {
			expects := []string{}
			if r.Expect.Status != 0 {
				expects = append(expects, fmt.Sprintf("status:%d", r.Expect.Status))
			}
			expects = append(expects, r.Expect.Buttons...)
			expects = append(expects, r.Expect.Fields...)
			expectStr := strings.Join(expects, ", ")
			if len(expectStr) > 25 {
				expectStr = expectStr[:25] + "..."
			}

			methodColor := lipgloss.Color("#00FFB3")
			if r.Method == "POST" {
				methodColor = lipgloss.Color("#FFD700")
			} else if r.Method == "DELETE" {
				methodColor = lipgloss.Color("#FF4444")
			}

			fmt.Printf("%-20s %-8s %-45s %s\n",
				styleHighlight.Render(r.Name),
				lipgloss.NewStyle().Foreground(methodColor).Render(r.Method),
				r.URL,
				styleInfo.Render(expectStr),
			)
		}
		fmt.Println()
		return nil
	},
}

var reqShowCmd = &cobra.Command{
	Use:   "show [nome]",
	Short: "Mostra detalhes de uma requisição",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := storage.Open()
		if err != nil {
			return err
		}
		defer db.Close()

		r, err := db.GetRequest(args[0])
		if err != nil {
			return err
		}

		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00FFB330")).
			Padding(1, 2)

		content := fmt.Sprintf("%s  %s\n\n", styleHighlight.Render(r.Method), r.URL)
		content += styleInfo.Render("Headers:\n")
		for k, v := range r.Headers {
			content += fmt.Sprintf("  %s: %s\n", styleHighlight.Render(k), v)
		}
		if r.Body != "" {
			content += styleInfo.Render("\nBody:\n") + "  " + r.Body + "\n"
		}
		if len(r.Expect.Buttons) > 0 {
			content += styleInfo.Render("\nEspera botões: ") + strings.Join(r.Expect.Buttons, ", ") + "\n"
		}
		if len(r.Expect.Fields) > 0 {
			content += styleInfo.Render("Espera campos: ") + strings.Join(r.Expect.Fields, ", ") + "\n"
		}
		if r.Expect.Status != 0 {
			content += styleInfo.Render("Espera status: ") + fmt.Sprintf("%d", r.Expect.Status) + "\n"
		}

		fmt.Println(box.Render(styleBrand.Render("⚡ "+r.Name) + "\n\n" + content))
		return nil
	},
}

var reqDeleteCmd = &cobra.Command{
	Use:   "delete [nome]",
	Short: "Remove uma requisição",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := storage.Open()
		if err != nil {
			return err
		}
		defer db.Close()

		if err := db.DeleteRequest(args[0]); err != nil {
			return err
		}
		fmt.Println(styleSuccess.Render("✓") + " Requisição " + styleHighlight.Render(args[0]) + " removida.")
		return nil
	},
}

func init() {
	reqAddCmd.Flags().String("name", "", "Nome da requisição (obrigatório)")
	reqAddCmd.Flags().String("url", "", "URL do endpoint (obrigatório)")
	reqAddCmd.Flags().String("method", "GET", "Método HTTP: GET, POST, PUT, DELETE, PATCH")
	reqAddCmd.Flags().String("body", "", "Body da requisição (JSON, form, etc)")
	reqAddCmd.Flags().StringArray("header", []string{}, "Header no formato Chave:Valor (repetível)")
	reqAddCmd.Flags().StringArray("expect-button", []string{}, "Nome de botão esperado na resposta")
	reqAddCmd.Flags().StringArray("expect-field", []string{}, "Nome de campo esperado na resposta")
	reqAddCmd.Flags().Int("expect-status", 0, "Status HTTP esperado")

	reqRunCmd.Flags().Bool("verbose", false, "Mostra headers da resposta")
	reqRunCmd.Flags().Bool("raw", false, "Mostra body sem formatação")

	_ = time.Now() // keep import

	requestCmd.AddCommand(reqAddCmd, reqRunCmd, reqListCmd, reqShowCmd, reqDeleteCmd)
}
