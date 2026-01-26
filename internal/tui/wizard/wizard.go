package wizard

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipglossv2 "charm.land/lipgloss/v2"
	"github.com/charmbracelet/lipgloss"
	uv "github.com/charmbracelet/ultraviolet"
)

// WizardResult holds the output values from the wizard.
// These are applied to buildFlags before orchestrator creation.
type WizardResult struct {
	SpecPath    string // Path to selected spec file
	Model       string // Selected model ID (e.g. "anthropic/claude-sonnet-4-5")
	Template    string // Full edited template content
	SessionName string // Validated session name
	Iterations  int    // Max iterations (0 = infinite)
}

// WizardModel is the main BubbleTea model for the build wizard.
// It manages the four-step flow: file picker → model selector → template editor → config.
type WizardModel struct {
	step      int          // Current step (0-3)
	cancelled bool         // User cancelled via ESC
	result    WizardResult // Accumulated result from each step
	width     int          // Terminal width
	height    int          // Terminal height

	// Step components
	filePickerStep     *FilePickerStep
	modelSelectorStep  *ModelSelectorStep
	templateEditorStep *TemplateEditorStep
	configStep         *ConfigStep
}

// RunWizard is the entry point for the build wizard.
// It creates a standalone BubbleTea program, runs it, and returns the result.
// Returns nil result and error if user cancels or an error occurs.
func RunWizard() (*WizardResult, error) {
	// Create initial model
	m := &WizardModel{
		step:      0,
		cancelled: false,
	}

	// Create BubbleTea program
	p := tea.NewProgram(m)

	// Run the program
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("wizard failed: %w", err)
	}

	// Extract result from final model
	wizModel, ok := finalModel.(*WizardModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}

	// Check if user cancelled
	if wizModel.cancelled {
		return nil, fmt.Errorf("wizard cancelled by user")
	}

	return &wizModel.result, nil
}

// Init initializes the wizard model.
func (m *WizardModel) Init() tea.Cmd {
	// Initialize file picker (step 0)
	m.filePickerStep = NewFilePickerStep()
	return nil
}

// Update handles messages for the wizard.
func (m *WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Global keybindings
		switch msg.String() {
		case "ctrl+c":
			// Always allow Ctrl+C to quit
			m.cancelled = true
			return m, tea.Quit
		case "esc":
			// ESC behavior depends on step
			if m.step == 0 {
				// On first step, exit wizard
				m.cancelled = true
				return m, tea.Quit
			} else {
				// On other steps, go back
				m.step--
				// Re-initialize the previous step if needed
				m.initCurrentStep()
				return m, nil
			}
		case "ctrl+enter":
			// Ctrl+Enter finishes wizard if all steps valid
			if m.isComplete() {
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		// Store terminal dimensions
		m.width = msg.Width
		m.height = msg.Height
		// Update size of current step
		m.updateCurrentStepSize()
		return m, nil

	case FileSelectedMsg:
		// File selected in step 0
		m.result.SpecPath = msg.Path
		m.step++
		m.initCurrentStep()
		return m, m.modelSelectorStep.Init()

	case ModelSelectedMsg:
		// Model selected in step 1
		m.result.Model = msg.ModelID
		m.step++
		m.initCurrentStep()
		return m, m.templateEditorStep.Init()

	case ConfigCompleteMsg:
		// Config complete in step 3
		m.result.Template = m.templateEditorStep.Content()
		m.result.SessionName = m.configStep.SessionName()
		m.result.Iterations = m.configStep.Iterations()
		return m, tea.Quit
	}

	// Forward to current step
	var cmd tea.Cmd
	switch m.step {
	case 0:
		if m.filePickerStep != nil {
			cmd = m.filePickerStep.Update(msg)
		}
	case 1:
		if m.modelSelectorStep != nil {
			cmd = m.modelSelectorStep.Update(msg)
		}
	case 2:
		if m.templateEditorStep != nil {
			cmd = m.templateEditorStep.Update(msg)
		}
		// Handle Enter to advance from template editor
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == "enter" {
			// Move to config step
			m.step++
			m.initCurrentStep()
			return m, m.configStep.Init()
		}
	case 3:
		if m.configStep != nil {
			cmd = m.configStep.Update(msg)
		}
	}

	return m, cmd
}

// initCurrentStep initializes the current step component if not already initialized.
func (m *WizardModel) initCurrentStep() {
	switch m.step {
	case 0:
		if m.filePickerStep == nil {
			m.filePickerStep = NewFilePickerStep()
		}
	case 1:
		if m.modelSelectorStep == nil {
			m.modelSelectorStep = NewModelSelectorStep()
		}
	case 2:
		if m.templateEditorStep == nil {
			m.templateEditorStep = NewTemplateEditorStep()
		}
	case 3:
		if m.configStep == nil {
			m.configStep = NewConfigStep(m.result.SpecPath)
		}
	}
	m.updateCurrentStepSize()
}

// updateCurrentStepSize updates the size of the current step component.
func (m *WizardModel) updateCurrentStepSize() {
	// Reserve space for modal container (padding, borders, title, buttons)
	contentWidth := m.width - 10
	contentHeight := m.height - 10
	if contentWidth < 40 {
		contentWidth = 40
	}
	if contentHeight < 10 {
		contentHeight = 10
	}

	switch m.step {
	case 0:
		if m.filePickerStep != nil {
			m.filePickerStep.SetSize(contentWidth, contentHeight)
		}
	case 1:
		if m.modelSelectorStep != nil {
			m.modelSelectorStep.SetSize(contentWidth, contentHeight)
		}
	case 2:
		if m.templateEditorStep != nil {
			m.templateEditorStep.SetSize(contentWidth, contentHeight)
		}
	case 3:
		if m.configStep != nil {
			m.configStep.SetSize(contentWidth, contentHeight)
		}
	}
}

// View renders the wizard UI.
func (m *WizardModel) View() tea.View {
	var view tea.View
	view.AltScreen = true
	view.KeyboardEnhancements = tea.KeyboardEnhancements{
		ReportEventTypes: true, // Required for ctrl+enter
	}

	// Render current step content
	var stepContent string
	switch m.step {
	case 0:
		if m.filePickerStep != nil {
			stepContent = m.filePickerStep.View()
		}
	case 1:
		if m.modelSelectorStep != nil {
			stepContent = m.modelSelectorStep.View()
		}
	case 2:
		if m.templateEditorStep != nil {
			stepContent = m.templateEditorStep.View()
		}
	case 3:
		if m.configStep != nil {
			stepContent = m.configStep.View()
		}
	}

	// Wrap in modal container with title
	content := m.renderModal(stepContent)

	canvas := uv.NewScreenBuffer(m.width, m.height)
	uv.NewStyledString(content).Draw(canvas, uv.Rectangle{
		Min: uv.Position{X: 0, Y: 0},
		Max: uv.Position{X: m.width, Y: m.height},
	})

	view.Content = lipglossv2.NewLayer(canvas.Render())
	return view
}

// renderModal wraps the step content in a modal container with title.
func (m *WizardModel) renderModal(stepContent string) string {
	var sections []string

	// Title with step indicator and step name
	stepNames := []string{
		"Select Spec File",
		"Select Model",
		"Edit Prompt Template",
		"Session Configuration",
	}
	title := fmt.Sprintf("Build Wizard - Step %d of 4: %s", m.step+1, stepNames[m.step])
	sections = append(sections, styleModalTitle.Render(title))
	sections = append(sections, "")

	// Step content
	sections = append(sections, stepContent)

	// Join all sections
	content := strings.Join(sections, "\n")

	// Calculate modal dimensions based on terminal size
	// Leave margins for visual spacing
	modalWidth := m.width - 10
	if modalWidth < 60 {
		modalWidth = 60
	}
	if modalWidth > 100 {
		modalWidth = 100 // Max width for readability
	}

	// Apply modal container style without fixed height
	// This allows content to determine height naturally
	modalStyle := styleModalContainer.Width(modalWidth)

	modalContent := modalStyle.Render(content)

	// Center the modal on screen
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		modalContent,
	)
}

// isComplete checks if all required steps have valid data.
func (m *WizardModel) isComplete() bool {
	// TODO: Validate each step
	return m.result.SpecPath != "" &&
		m.result.Model != "" &&
		m.result.Template != "" &&
		m.result.SessionName != ""
}
