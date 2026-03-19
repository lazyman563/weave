package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/lazyman563/weave/internal/proxy"
	"github.com/lazyman563/weave/internal/storage"
)

var flowCmd = &cobra.Command{
	Use:   "flow",
	Short: "Criar e executar fluxos de requisições encadeadas",
	Long: `Cria e executa fluxos de requisições encadeadas.
O output de uma requisição pode ser usado como input da próxima.

SUBCOMANDOS:
  add   Cria um novo flow
  run   Executa um flow
  list  Lista flows salvos

EXEMPLOS:
  weave flow add --name "login-completo" --chain "login,get-profile,logout" --delay 300
  weave flow run "login-completo"
  weave flow run --chain "login,get-profile" --delay 500`,
}

var flowAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Cria um flow encadeado",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		chain, _ := cmd.Flags().GetString("chain")
		delay, _ := cmd.Flags().GetInt("delay")

		if name == "" || chain == "" {
			return fmt.Errorf("--name e --chain são obrigatórios")
		}

		steps := strings.Split(chain, ",")
		for i, s := range steps {
			steps[i] = strings.TrimSpace(s)
		}

		db, err := storage.Open()
		if err != nil {
			return err
		}
		defer db.Close()

		if err := db.SaveFlow(&storage.Flow{Name: name, Chain: steps, Delay: delay}); err != nil {
			return err
		}

		fmt.Printf("%s Flow %s salvo (%d steps)\n",
			styleSuccess.Render("✓"),
			styleHighlight.Render(name),
			len(steps))
		return nil
	},
}

var flowRunCmd = &cobra.Command{
	Use:   "run [nome]",
	Short: "Executa um flow salvo ou inline via --chain",
	Example: `  weave flow run "login-completo"
  weave flow run --chain "login,perfil" --delay 200`,
	RunE: func(cmd *cobra.Command, args []string) error {
		inlineChain, _ := cmd.Flags().GetString("chain")
		delay, _ := cmd.Flags().GetInt("delay")

		db, err := storage.Open()
		if err != nil {
			return err
		}
		defer db.Close()

		var steps []string

		if inlineChain != "" {
			for _, s := range strings.Split(inlineChain, ",") {
				steps = append(steps, strings.TrimSpace(s))
			}
		} else if len(args) > 0 {
			flow, err := db.GetFlow(args[0])
			if err != nil {
				return err
			}
			steps = flow.Chain
			if delay == 0 {
				delay = flow.Delay
			}
		} else {
			return fmt.Errorf("informe o nome do flow ou use --chain")
		}

		fmt.Printf("\n⚡ Executando flow: %s\n", styleHighlight.Render(strings.Join(steps, " → ")))
		fmt.Println(styleInfo.Render(strings.Repeat("─", 60)))

		var lastBody string
		for i, stepName := range steps {
			r, err := db.GetRequest(stepName)
			if err != nil {
				fmt.Printf("%s [%d/%d] %s — %s\n",
					styleError.Render("✗"), i+1, len(steps),
					styleHighlight.Render(stepName),
					styleError.Render(err.Error()))
				continue
			}

			// inject previous body as context if needed
			if lastBody != "" && strings.Contains(r.Body, "{{previous}}") {
				r.Body = strings.ReplaceAll(r.Body, "{{previous}}", lastBody)
			}

			result, _ := proxy.RunRequest(r)

			statusColor := styleSuccess
			if result.StatusCode >= 400 {
				statusColor = styleError
			}

			fmt.Printf("%s [%d/%d] %-20s %s  %dms\n",
				statusColor.Render("●"),
				i+1, len(steps),
				styleHighlight.Render(stepName),
				statusColor.Render(fmt.Sprintf("HTTP %d", result.StatusCode)),
				result.Duration.Milliseconds(),
			)

			if len(result.Violations) > 0 {
				for _, v := range result.Violations {
					fmt.Printf("         %s %s\n", styleError.Render("↳"), v.Message)
				}
			}

			lastBody = result.Body

			// save history
			db.SaveHistory(&storage.HistoryEntry{
				RequestID:  r.ID,
				Name:       r.Name,
				URL:        r.URL,
				Method:     r.Method,
				StatusCode: result.StatusCode,
				Body:       result.Body,
				Duration:   result.Duration.Milliseconds(),
				Error:      result.Error,
			})

			if delay > 0 && i < len(steps)-1 {
				fmt.Printf("         %s aguardando %dms...\n", styleInfo.Render("⏱"), delay)
				time.Sleep(time.Duration(delay) * time.Millisecond)
			}
		}

		fmt.Println(styleInfo.Render(strings.Repeat("─", 60)))
		fmt.Printf("%s Flow concluído.\n\n", styleSuccess.Render("✓"))
		return nil
	},
}

var flowListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista todos os flows salvos",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := storage.Open()
		if err != nil {
			return err
		}
		defer db.Close()

		flows, err := db.ListFlows()
		if err != nil {
			return err
		}

		if len(flows) == 0 {
			fmt.Println(styleInfo.Render("Nenhum flow salvo. Use: weave flow add"))
			return nil
		}

		for _, f := range flows {
			fmt.Printf("  %s  %s  %s\n",
				styleHighlight.Render(f.Name),
				styleInfo.Render("→"),
				strings.Join(f.Chain, styleInfo.Render(" → ")),
			)
			if f.Delay > 0 {
				fmt.Printf("        %s\n", styleInfo.Render(fmt.Sprintf("delay: %dms", f.Delay)))
			}
		}
		return nil
	},
}

func init() {
	flowAddCmd.Flags().String("name", "", "Nome do flow")
	flowAddCmd.Flags().String("chain", "", "Steps separados por vírgula: login,perfil,logout")
	flowAddCmd.Flags().Int("delay", 0, "Delay em ms entre steps")

	flowRunCmd.Flags().String("chain", "", "Executa flow inline sem salvar")
	flowRunCmd.Flags().Int("delay", 0, "Delay entre steps (ms)")

	flowCmd.AddCommand(flowAddCmd, flowRunCmd, flowListCmd)
}
