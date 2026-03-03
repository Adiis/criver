package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/adiis/criver/internal/browser"
	"github.com/adiis/criver/internal/chrome"
)

// TUI states.
type state int

const (
	stateLoading state = iota
	stateList
	stateSearch
	stateSearchResults
	stateDownloading
	statePathPrompt
	stateDone
)

// Messages.
type versionsLoaded struct {
	data     chrome.VersionData
	browsers []browser.Installed
	err      error
}

type downloadComplete struct {
	err error
}

type pathAppended struct {
	rcFile string
	err    error
}

// List items.
type versionItem struct {
	version     string
	recommended string
}

func (v versionItem) Title() string {
	title := "chromedriver " + v.version
	if v.recommended != "" {
		title += " (recommended)"
	}
	return title
}

func (v versionItem) Description() string {
	if v.recommended != "" {
		return v.recommended
	}
	if idx := strings.IndexByte(v.version, '.'); idx > 0 {
		return "Major version " + v.version[:idx]
	}
	return v.version
}

func (v versionItem) FilterValue() string { return v.version }

type searchAction struct{}

func (s searchAction) Title() string       { return "Search for a specific version..." }
func (s searchAction) Description() string { return "Type a version number to find and install" }
func (s searchAction) FilterValue() string { return "search" }

// Model is the Bubbletea model for the TUI.
type Model struct {
	state       state
	platform    string
	allVersions []string
	list        list.Model
	spinner     spinner.Model
	textInput   textinput.Model
	selected    string
	message     string
	err         error
	windowWidth int
	pathCursor  int
	rcFile      string
}

// NewModel creates the initial TUI model.
func NewModel(platform string) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	ti := textinput.New()
	ti.Placeholder = "e.g. 131 or 131.0.6778"
	ti.CharLimit = 30
	ti.Width = 40

	return Model{
		state:     stateLoading,
		platform:  platform,
		spinner:   s,
		textInput: ti,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchVersionsCmd())
}

func fetchVersionsCmd() tea.Cmd {
	return func() tea.Msg {
		browsers := browser.DetectInstalled()
		data, err := chrome.FetchVersions()
		return versionsLoaded{data: data, browsers: browsers, err: err}
	}
}

func downloadCmd(version, platform string) tea.Cmd {
	return func() tea.Msg {
		err := chrome.DownloadAndInstall(version, platform)
		return downloadComplete{err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Search input state handles its own keys.
		if m.state == stateSearch {
			return m.updateSearch(msg)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.state != stateDownloading {
				return m, tea.Quit
			}
		case "/":
			if m.state == stateList {
				m.state = stateSearch
				m.textInput.SetValue("")
				m.textInput.Focus()
				return m, m.textInput.Cursor.BlinkCmd()
			}
		case "enter":
			if m.state == stateList {
				sel := m.list.SelectedItem()
				if _, ok := sel.(searchAction); ok {
					m.state = stateSearch
					m.textInput.SetValue("")
					m.textInput.Focus()
					return m, m.textInput.Cursor.BlinkCmd()
				}
				if item, ok := sel.(versionItem); ok {
					m.selected = item.version
					m.state = stateDownloading
					return m, tea.Batch(m.spinner.Tick, downloadCmd(item.version, m.platform))
				}
			}
			if m.state == stateSearchResults {
				if item, ok := m.list.SelectedItem().(versionItem); ok {
					m.selected = item.version
					m.state = stateDownloading
					return m, tea.Batch(m.spinner.Tick, downloadCmd(item.version, m.platform))
				}
			}
			if m.state == statePathPrompt {
				if m.pathCursor == 0 {
					rcFile := m.rcFile
					return m, func() tea.Msg {
						err := chrome.AppendToPath(rcFile)
						return pathAppended{rcFile: rcFile, err: err}
					}
				}
				m.state = stateDone
				m.message += fmt.Sprintf("\n\nManually add to PATH:\n  export PATH=\"%s:$PATH\"", chrome.InstallDir)
				return m, nil
			}
			if m.state == stateDone {
				return m, tea.Quit
			}
		case "left", "h":
			if m.state == statePathPrompt && m.pathCursor > 0 {
				m.pathCursor--
			}
		case "right", "l":
			if m.state == statePathPrompt && m.pathCursor < 1 {
				m.pathCursor++
			}
		case "tab":
			if m.state == statePathPrompt {
				m.pathCursor = (m.pathCursor + 1) % 2
			}
		case "y", "Y":
			if m.state == statePathPrompt {
				m.pathCursor = 0
			}
		case "n", "N":
			if m.state == statePathPrompt {
				m.pathCursor = 1
			}
		}

	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		if m.state == stateList || m.state == stateSearchResults {
			m.list.SetSize(msg.Width, msg.Height-2)
		}

	case versionsLoaded:
		if msg.err != nil {
			m.state = stateDone
			m.err = msg.err
			m.message = fmt.Sprintf("Error: %v", msg.err)
			return m, nil
		}
		m.allVersions = msg.data.All
		m.state = stateList

		browserByMajor := make(map[int]browser.Installed)
		for _, b := range msg.browsers {
			browserByMajor[b.Major] = b
		}

		var recommended []list.Item
		var others []list.Item
		for _, v := range msg.data.Top {
			major := chrome.ParseMajor(v)
			item := versionItem{version: v}
			if b, ok := browserByMajor[major]; ok {
				item.recommended = fmt.Sprintf("matches installed %s %s", b.Name, b.Version)
				recommended = append(recommended, item)
			} else {
				others = append(others, item)
			}
		}
		var items []list.Item
		items = append(items, searchAction{})
		items = append(items, recommended...)
		items = append(items, others...)

		m.list = m.newList(items, "Criver — Select chromedriver version")
		return m, nil

	case downloadComplete:
		if msg.err != nil {
			m.state = stateDone
			m.err = msg.err
			m.message = fmt.Sprintf("Error installing chromedriver %s: %v", m.selected, msg.err)
			return m, nil
		}
		if !chrome.IsInPath() {
			m.rcFile = chrome.DetectShellRC()
			if m.rcFile != "" {
				m.state = statePathPrompt
				m.message = fmt.Sprintf("Installed chromedriver %s to %s/chromedriver", m.selected, chrome.InstallDir)
				return m, nil
			}
		}
		m.state = stateDone
		m.message = fmt.Sprintf("Installed chromedriver %s to %s/chromedriver", m.selected, chrome.InstallDir)
		return m, nil

	case pathAppended:
		m.state = stateDone
		if msg.err != nil {
			m.message += fmt.Sprintf("\n\nFailed to update %s: %v", msg.rcFile, msg.err)
			m.message += fmt.Sprintf("\nManually add:\n  export PATH=\"%s:$PATH\"", chrome.InstallDir)
		} else {
			m.message += fmt.Sprintf("\n\nAdded %s to PATH in %s", chrome.InstallDir, msg.rcFile)
			m.message += "\nRestart your shell or run: source " + msg.rcFile
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	if m.state == stateList || m.state == stateSearchResults {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		// Go back to main list.
		m.state = stateList
		m.textInput.Blur()
		return m, nil
	case "enter":
		query := strings.TrimSpace(m.textInput.Value())
		if query == "" {
			return m, nil
		}
		matches := chrome.SearchVersions(m.allVersions, query, 20)
		if len(matches) == 0 {
			m.message = fmt.Sprintf("No versions found matching \"%s\"", query)
			m.state = stateDone
			m.textInput.Blur()
			return m, nil
		}
		items := make([]list.Item, len(matches))
		for i, v := range matches {
			items[i] = versionItem{version: v}
		}
		m.list = m.newList(items, fmt.Sprintf("Results for \"%s\" — %d found", query, len(matches)))
		m.state = stateSearchResults
		m.textInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) newList(items []list.Item, title string) list.Model {
	delegate := list.NewDefaultDelegate()
	height := 14
	if len(items) > 10 {
		height = 24
	}
	l := list.New(items, delegate, 80, height)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	return l
}

func (m Model) View() string {
	switch m.state {
	case stateLoading:
		return fmt.Sprintf("\n  %s Fetching chromedriver versions...\n", m.spinner.View())
	case stateList, stateSearchResults:
		v := m.list.View()
		if m.state == stateSearchResults {
			v += "\n  Press q to quit or esc to go back (via list)."
		}
		return v
	case stateSearch:
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Padding(1, 2)
		inputStyle := lipgloss.NewStyle().Padding(0, 2)
		hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(1, 2)

		s := titleStyle.Render("Search chromedriver version") + "\n"
		s += inputStyle.Render(m.textInput.View()) + "\n"
		s += hintStyle.Render("Enter to search / Esc to go back")
		return s
	case stateDownloading:
		return fmt.Sprintf("\n  %s Downloading chromedriver %s for %s...\n", m.spinner.View(), m.selected, m.platform)
	case statePathPrompt:
		successStyle := lipgloss.NewStyle().Padding(1, 2).Foreground(lipgloss.Color("82"))
		promptStyle := lipgloss.NewStyle().Padding(0, 2)
		selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Background(lipgloss.Color("236")).Padding(0, 1)
		normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)

		yes, no := normalStyle.Render("Yes"), normalStyle.Render("No")
		if m.pathCursor == 0 {
			yes = selectedStyle.Render("Yes")
		} else {
			no = selectedStyle.Render("No")
		}

		s := successStyle.Render(m.message) + "\n\n"
		s += promptStyle.Render(fmt.Sprintf(
			"%s is not in your PATH. Add it to %s?\n\n  %s  %s",
			chrome.InstallDir, m.rcFile, yes, no,
		)) + "\n"
		return s
	case stateDone:
		style := lipgloss.NewStyle().Padding(1, 2)
		if m.err != nil {
			return style.Foreground(lipgloss.Color("196")).Render(m.message) + "\n"
		}
		return style.Foreground(lipgloss.Color("82")).Render(m.message) + "\n\n  Press enter to exit.\n"
	}
	return ""
}
