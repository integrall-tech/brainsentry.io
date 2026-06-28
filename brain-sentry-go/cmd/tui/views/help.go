package views

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/integraltech/brainsentry/cmd/tui/theme"
)

// HelpView returns the help overlay content.
func HelpView(width, height int) string {
	helpKeyStyle := lipgloss.NewStyle().
		Foreground(theme.Accent).
		Bold(true).
		Width(12)

	helpDescStyle := lipgloss.NewStyle().
		Foreground(theme.Text)

	helpEntry := func(key, desc string) string {
		return helpKeyStyle.Render(key) + helpDescStyle.Render(desc) + "\n"
	}

	title := theme.TitleStyle.MarginBottom(1).Render("BrainSentry TUI - Keybindings")

	nav := theme.SectionStyle.Render("Navigation")
	navKeys := helpEntry("1-5", "Switch view") +
		helpEntry("j/k", "Move down/up") +
		helpEntry("g/G", "Top/bottom") +
		helpEntry("Enter", "Select/open") +
		helpEntry("Esc", "Back/cancel") +
		helpEntry("Tab", "Next field") +
		helpEntry("Ctrl+D/U", "Page down/up")

	actions := theme.SectionStyle.Render("Actions")
	actionKeys := helpEntry("/", "Search") +
		helpEntry("n", "New item") +
		helpEntry("e", "Edit") +
		helpEntry("d", "Delete") +
		helpEntry("Ctrl+S", "Save") +
		helpEntry("r", "Relationships")

	global := theme.SectionStyle.Render("Global")
	globalKeys := helpEntry("?", "Toggle help") +
		helpEntry("q/Ctrl+C", "Quit")

	views := theme.SectionStyle.Render("Views")
	viewKeys := helpEntry("1", "Dashboard") +
		helpEntry("2", "Memories") +
		helpEntry("3", "Search") +
		helpEntry("4", "Sessions") +
		helpEntry("5", "Relationships")

	content := title + "\n\n" +
		nav + "\n" + navKeys + "\n" +
		actions + "\n" + actionKeys + "\n" +
		global + "\n" + globalKeys + "\n" +
		views + "\n" + viewKeys

	box := theme.ActiveCardStyle.Render(content)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
