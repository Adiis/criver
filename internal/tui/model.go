package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/adis/criver/internal/browser"
	"github.com/adis/criver/internal/chrome"
)

// TUI states.
type state int

const (
	stateLoading state = iota
	stateList
	stateDownloading
	statePathPrompt
	stateDone
)

// Messages.
type versionsLoaded struct {
	versions []string
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

// List item.
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

// Model is the Bubbletea model for the TUI.
type Model struct {
	state       state
	platform    string
	versions    []string
	list        list.Model
	spinner     spinner.Model
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

	return Model{
		state:    stateLoading,
		platform: platform,
		spinner:  s,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchVersionsCmd())
}

func fetchVersionsCmd() tea.Cmd {
	return func() tea.Msg {
		browsers := browser.DetectInstalled()
		versions, err := chrome.FetchTopVersions()
		return versionsLoaded{versions: versions, browsers: browsers, err: err}
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
		switch msg.String() {
		case "ctrl+c", "q":
			if m.state != stateDownloading {
				return m, tea.Quit
			}
		case "enter":
			if m.state == stateList {
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
		if m.state == stateList {
			m.list.SetSize(msg.Width, msg.Height-2)
		}

	case versionsLoaded:
		if msg.err != nil {
			m.state = stateDone
			m.err = msg.err
			m.message = fmt.Sprintf("Error: %v", msg.err)
			return m, nil
		}
		m.versions = msg.versions
		m.state = stateList

		browserByMajor := make(map[int]browser.Installed)
		for _, b := range msg.browsers {
			browserByMajor[b.Major] = b
		}

		var recommended []list.Item
		var others []list.Item
		for _, v := range msg.versions {
			major := chrome.ParseMajor(v)
			item := versionItem{version: v}
			if b, ok := browserByMajor[major]; ok {
				item.recommended = fmt.Sprintf("matches installed %s %s", b.Name, b.Version)
				recommended = append(recommended, item)
			} else {
				others = append(others, item)
			}
		}
		items := append(recommended, others...)

		delegate := list.NewDefaultDelegate()
		l := list.New(items, delegate, 80, 14)
		l.Title = "Criver — Select chromedriver version"
		l.SetShowStatusBar(false)
		l.SetFilteringEnabled(false)
		l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
		m.list = l
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

	if m.state == stateList {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	switch m.state {
	case stateLoading:
		return fmt.Sprintf("\n  %s Fetching chromedriver versions...\n", m.spinner.View())
	case stateList:
		return m.list.View()
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
