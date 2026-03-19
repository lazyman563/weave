package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	styleBrand = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FFB3")).
			Background(lipgloss.Color("#0A0A0A")).
			Padding(0, 1)

	styleVersion = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			Italic(true)

	styleError = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF4444"))

	styleSuccess = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FFB3"))

	styleInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	styleHighlight = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFD700"))
)

var rootCmd = &cobra.Command{
	Use:   "weave",
	Short: "⚡ weave — HTTP client with memory, logic, and a browser",
	Long: `
` + styleBrand.Render("⚡ WEAVE") + ` ` + styleVersion.Render("v1.0.0") + `

A heavyweight HTTP client for the terminal.
Save requests, chain flows, inspect pages visually, and fix them with AI.

` + styleHighlight.Render("SUBCOMMANDS:") + `
  weave request     Manage and run HTTP requests
  weave flow        Build and run chained request flows  
  weave history     View request/response history
  weave serve       Start the visual browser proxy
  weave fix         AI-powered page element fixer
  weave config      Manage .weaverc configuration

` + styleHighlight.Render("EXAMPLES:") + `
  weave request add --name "login" --url https://api.site.com/auth --method POST
  weave request run "login"
  weave flow run --chain "login>profile>logout"
  weave serve --target https://site.com
  weave fix --element "#submit-btn" --instruction "deve chamar /auth/v2"

` + styleInfo.Render("Config file: ~/.weaverc | Docs: weave --help <subcommand>"),
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, styleError.Render("ERRO: "+err.Error()))
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(requestCmd)
	rootCmd.AddCommand(flowCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(fixCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
