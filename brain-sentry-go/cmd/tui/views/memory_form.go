package views

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/integraltech/brainsentry/cmd/tui/components"
	"github.com/integraltech/brainsentry/cmd/tui/theme"
	"github.com/integraltech/brainsentry/internal/client"
	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/dto"
)

// MemorySavedMsg is sent when a memory is saved.
type MemorySavedMsg struct{ Memory *dto.MemoryResponse }

// MemorySaveErrorMsg is sent when saving fails.
type MemorySaveErrorMsg struct{ Err error }

// MemoryFormModel handles creating and editing memories.
type MemoryFormModel struct {
	client  *client.Client
	editID  string
	form    *huh.Form
	toast   components.Toast
	saving  bool
	loading bool
	width   int
	height  int

	// form value bindings
	content  string
	summary  string
	tags     string
	category string
	confirm  bool
}

// NewMemoryFormModel creates a new memory form.
func NewMemoryFormModel(c *client.Client, editID string) MemoryFormModel {
	m := MemoryFormModel{
		client:   c,
		editID:   editID,
		category: string(domain.CategoryKnowledge),
	}
	m.form = m.buildForm()
	return m
}

func (m *MemoryFormModel) buildForm() *huh.Form {
	title := "New Memory"
	if m.editID != "" {
		title = "Edit Memory"
	}

	categoryOptions := []huh.Option[string]{
		huh.NewOption("Knowledge", string(domain.CategoryKnowledge)),
		huh.NewOption("Insight", string(domain.CategoryInsight)),
		huh.NewOption("Decision", string(domain.CategoryDecision)),
		huh.NewOption("Warning", string(domain.CategoryWarning)),
		huh.NewOption("Pattern", string(domain.CategoryPattern)),
		huh.NewOption("Anti-Pattern", string(domain.CategoryAntipattern)),
		huh.NewOption("Bug", string(domain.CategoryBug)),
		huh.NewOption("Optimization", string(domain.CategoryOptimization)),
		huh.NewOption("Context", string(domain.CategoryContext)),
		huh.NewOption("Reference", string(domain.CategoryReference)),
		huh.NewOption("Action", string(domain.CategoryAction)),
		huh.NewOption("Domain", string(domain.CategoryDomain)),
		huh.NewOption("Integration", string(domain.CategoryIntegration)),
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title("Content").
				Key("content").
				Value(&m.content).
				Placeholder("Describe the knowledge, insight, or pattern...").
				Lines(8).
				CharLimit(10000),

			huh.NewInput().
				Title("Summary").
				Key("summary").
				Value(&m.summary).
				Placeholder("Brief summary (optional)").
				CharLimit(500),

			huh.NewInput().
				Title("Tags").
				Key("tags").
				Value(&m.tags).
				Placeholder("tag1, tag2, tag3").
				CharLimit(200),

			huh.NewSelect[string]().
				Title("Category").
				Key("category").
				Value(&m.category).
				Options(categoryOptions...).
				Height(8),

			huh.NewConfirm().
				Title("Save this memory?").
				Key("confirm").
				Value(&m.confirm).
				Affirmative("Save").
				Negative("Cancel"),
		).Title(title),
	).WithTheme(huh.ThemeCatppuccin()).
		WithWidth(min(80, m.width-10))
}

// Init initializes the form. If editing, loads the existing memory.
func (m MemoryFormModel) Init() tea.Cmd {
	if m.editID != "" {
		c := m.client
		id := m.editID
		return func() tea.Msg {
			memory, err := c.GetMemory(id)
			if err != nil {
				return MemorySaveErrorMsg{Err: err}
			}
			return MemoryDetailLoadedMsg{Memory: memory}
		}
	}
	return m.form.Init()
}

// Update handles messages for the memory form.
func (m MemoryFormModel) Update(msg tea.Msg) (MemoryFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case MemoryDetailLoadedMsg:
		// Populate form with existing memory data
		mem := msg.Memory
		m.content = mem.Content
		m.summary = mem.Summary
		m.tags = strings.Join(mem.Tags, ", ")
		m.category = string(mem.Category)
		m.loading = false
		m.form = m.buildForm()
		return m, m.form.Init()

	case MemorySavedMsg:
		m.saving = false
		return m, nil // AppModel handles MemorySavedMsg

	case MemorySaveErrorMsg:
		m.saving = false
		cmd := m.toast.Show("Save failed: "+msg.Err.Error(), components.ToastError)
		m.form = m.buildForm()
		initCmd := m.form.Init()
		return m, tea.Batch(cmd, initCmd)

	case components.ToastDismissMsg:
		m.toast.Dismiss()
		return m, nil
	}

	if m.saving || m.loading {
		return m, nil
	}

	// Forward to huh form
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	// Check if form completed
	if m.form.State == huh.StateCompleted {
		if !m.confirm {
			return m, func() tea.Msg { return NavigateBackMsg{} }
		}
		return m, m.save()
	}

	if m.form.State == huh.StateAborted {
		return m, func() tea.Msg { return NavigateBackMsg{} }
	}

	return m, cmd
}

func (m *MemoryFormModel) save() tea.Cmd {
	content := m.content
	if strings.TrimSpace(content) == "" {
		cmd := m.toast.Show("Content is required", components.ToastWarning)
		return cmd
	}

	m.saving = true
	c := m.client
	editID := m.editID
	category := domain.MemoryCategory(m.category)
	summary := m.summary

	var tags []string
	if m.tags != "" {
		for _, tag := range strings.Split(m.tags, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}

	return func() tea.Msg {
		if editID != "" {
			resp, err := c.UpdateMemory(editID, &dto.UpdateMemoryRequest{
				Content:  content,
				Summary:  summary,
				Category: category,
				Tags:     tags,
			})
			if err != nil {
				return MemorySaveErrorMsg{Err: err}
			}
			return MemorySavedMsg{Memory: resp}
		}

		resp, err := c.CreateMemory(&dto.CreateMemoryRequest{
			Content:  content,
			Summary:  summary,
			Category: category,
			Tags:     tags,
		})
		if err != nil {
			return MemorySaveErrorMsg{Err: err}
		}
		return MemorySavedMsg{Memory: resp}
	}
}

// View renders the memory form.
func (m MemoryFormModel) View() string {
	if m.loading {
		return lipgloss.Place(m.width, m.height-1,
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(theme.Primary).Render("Loading memory..."),
		)
	}

	if m.saving {
		return lipgloss.Place(m.width, m.height-1,
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render("Saving..."),
		)
	}

	content := m.form.View()
	toast := m.toast.View()
	if toast != "" {
		content = toast + "\n" + content
	}

	return "\n" + content
}
