package views

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/integraltech/brainsentry/cmd/tui/theme"
	"github.com/integraltech/brainsentry/internal/client"
	"github.com/integraltech/brainsentry/internal/dto"
)

// LoginSuccessMsg is sent when login succeeds.
type LoginSuccessMsg struct {
	Response *dto.LoginResponse
}

// LoginErrorMsg is sent when login fails.
type LoginErrorMsg struct {
	Err error
}

const (
	loginPhaseMode  = 0 // choosing login mode
	loginPhaseCreds = 1 // entering credentials
)

// LoginModel handles the login view.
type LoginModel struct {
	client  *client.Client
	form    *huh.Form
	phase   int
	mode    string
	email   string
	pass    string
	err     string
	loading bool
	width   int
	height  int
}

// NewLoginModel creates a new login view.
func NewLoginModel(c *client.Client) LoginModel {
	m := LoginModel{
		client: c,
		mode:   "credentials",
		phase:  loginPhaseMode,
	}
	m.form = m.buildModeForm()
	return m
}

func (m *LoginModel) buildModeForm() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("How would you like to log in?").
				Description("Demo: demo@example.com / demo123").
				Key("mode").
				Options(
					huh.NewOption("Demo Login  (demo@example.com / demo123)", "demo"),
					huh.NewOption("Login with email & password", "credentials"),
				).
				Height(4),
		),
	).WithTheme(huh.ThemeCatppuccin()).
		WithWidth(55)
}

func (m *LoginModel) buildCredsForm() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Email").
				Key("email").
				Placeholder("email@example.com").
				CharLimit(100).
				Validate(func(s string) error {
					if s == "" {
						return errRequired("email")
					}
					return nil
				}),

			huh.NewInput().
				Title("Password").
				Key("password").
				Placeholder("••••••••").
				EchoMode(huh.EchoModePassword).
				CharLimit(100).
				Validate(func(s string) error {
					if s == "" {
						return errRequired("password")
					}
					return nil
				}),
		),
	).WithTheme(huh.ThemeCatppuccin()).
		WithWidth(55)
}

type errRequired string

func (e errRequired) Error() string {
	return string(e) + " is required"
}

// Init initializes the login model.
func (m LoginModel) Init() tea.Cmd {
	return m.form.Init()
}

// Update handles messages for the login view.
func (m LoginModel) Update(msg tea.Msg) (LoginModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case LoginSuccessMsg:
		m.loading = false
		return m, nil

	case LoginErrorMsg:
		m.loading = false
		m.err = msg.Err.Error()
		// Rebuild form for retry
		if m.phase == loginPhaseCreds {
			m.form = m.buildCredsForm()
		} else {
			m.form = m.buildModeForm()
			m.phase = loginPhaseMode
		}
		return m, m.form.Init()
	}

	if m.loading {
		return m, nil
	}

	// Forward to huh form
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		switch m.phase {
		case loginPhaseMode:
			// Read value directly from form instead of binding
			selectedMode, _ := m.form.Get("mode").(string)
			if selectedMode == "demo" {
				m.email = "demo@example.com"
				m.pass = "demo123"
				return m, m.doLogin()
			}
			// Credentials: show email/password form
			m.phase = loginPhaseCreds
			m.form = m.buildCredsForm()
			return m, m.form.Init()

		case loginPhaseCreds:
			// Read values directly from form
			m.email, _ = m.form.Get("email").(string)
			m.pass, _ = m.form.Get("password").(string)
			return m, m.doLogin()
		}
	}

	if m.form.State == huh.StateAborted {
		if m.phase == loginPhaseCreds {
			// Go back to mode selection
			m.phase = loginPhaseMode
			m.form = m.buildModeForm()
			m.err = ""
			return m, m.form.Init()
		}
		return m, tea.Quit
	}

	return m, cmd
}

func (m *LoginModel) doLogin() tea.Cmd {
	m.loading = true
	m.err = ""
	c := m.client
	email := m.email
	pass := m.pass
	return func() tea.Msg {
		resp, err := c.Login(email, pass)
		if err != nil {
			return LoginErrorMsg{Err: err}
		}
		return LoginSuccessMsg{Response: resp}
	}
}

// View renders the login form.
func (m LoginModel) View() string {
	logoStyle := lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)
	logo := logoStyle.Render(`
 ____            _       ____             _
| __ ) _ __ __ _(_)_ __ / ___|  ___ _ __ | |_ _ __ _   _
|  _ \| '__/ _  | | '_ \\___ \ / _ \ '_ \| __| '__| | | |
| |_) | | | (_| | | | | |___) |  __/ | | | |_| |  | |_| |
|____/|_|  \__,_|_|_| |_|____/ \___|_| |_|\__|_|   \__, |
                                                    |___/ `)

	var content string
	if m.loading {
		loadingStyle := lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)
		content = logo + "\n\n" + loadingStyle.Render("  Authenticating...")
	} else {
		content = logo + "\n\n" + m.form.View()
	}

	if m.err != "" {
		content += "\n" + theme.ErrorStyle.Render("  "+m.err)
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		Padding(1, 3).
		Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}
