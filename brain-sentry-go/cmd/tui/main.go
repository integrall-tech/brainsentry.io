package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/integraltech/brainsentry/cmd/tui/views"
	"github.com/integraltech/brainsentry/internal/client"
)

func main() {
	// Configuration defaults
	serverURL := "http://localhost:8081/api"
	tenantID := "default"
	tokenFile := "~/.brainsentry-token"

	// Environment overrides
	if v := os.Getenv("BRAINSENTRY_URL"); v != "" {
		serverURL = v
	}
	if v := os.Getenv("BRAINSENTRY_TENANT"); v != "" {
		tenantID = v
	}
	if v := os.Getenv("BRAINSENTRY_TOKEN_FILE"); v != "" {
		tokenFile = v
	}

	// Create HTTP client
	c := client.New(serverURL, tenantID)

	// Try to load saved token for auto-login. A failure here just means no
	// saved session; any loaded token is validated on the first API call.
	_ = c.LoadToken(tokenFile)

	// Create and run TUI
	app := NewAppModel(c)

	// If we have a valid token, skip login
	if c.IsAuthenticated() {
		app.activeView = ViewDashboard
		app.dashboard = views.NewDashboardModel(c)
	}

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
