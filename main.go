package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/timer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gen2brain/beeep"
)

var (
	focusedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle   = focusedStyle.Copy()
	noStyle       = lipgloss.NewStyle()
	focusedButton = focusedStyle.Copy().Render("[ Submit ]")
	blurredButton = fmt.Sprintf("[ %s ]", blurredStyle.Render("Submit"))
)

type model struct {
	currentTime    time.Duration
	focusTime      int
	percent        float64
	breakTime      int
	longBreak      int
	currentTask    string
	sessionCounter int
	focusCounter   int
	timer          timer.Model
	keymap         keymap
	help           help.Model
	active         bool
	progress       progress.Model
	settingsActive bool
	inputs         []textinput.Model
	focusIndex     int
	warningMessage string
}

type keymap struct {
	start        key.Binding
	skip         key.Binding
	resume       key.Binding
	stop         key.Binding
	reset        key.Binding
	settings     key.Binding
	saveSettings key.Binding
	cancel       key.Binding
	quit         key.Binding
}

const (
	maxWidth = 50
)

func main() {
	m := initModel()
	m.keymap.resume.SetEnabled(false)
	m.keymap.skip.SetEnabled(false)
	m.keymap.saveSettings.SetEnabled(false)
	m.keymap.cancel.SetEnabled(false)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Uh oh, we encountered an error:", err)
		os.Exit(1)
	}
}

func (m model) Init() tea.Cmd {
	m.timer.Init()
	return m.timer.Stop()
}
func (m *model) settingsModel() {
	m.inputs = make([]textinput.Model, 3)
	var t textinput.Model
	for i := range m.inputs {
		t = textinput.New()
		t.Cursor.Style = cursorStyle
		t.CharLimit = 32

		switch i {
		case 0:
			t.Placeholder = fmt.Sprint(m.focusTime)
			t.Prompt = "Focus Time > "
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle
		case 1:
			t.Placeholder = fmt.Sprint(m.breakTime)
			t.Prompt = "Break Time > "
		case 2:
			t.Placeholder = fmt.Sprint(m.longBreak)
			t.Prompt = "Long Break Time > "
		}

		m.inputs[i] = t
	}

}
func (m *model) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	// Only text inputss with Focus() set will respond, so it's safe to simply
	// update all of them here without any further logic.

	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}
func (m *model) Resume() (*model, tea.Cmd) {
	m.keymap.saveSettings.SetEnabled(false)
	m.keymap.cancel.SetEnabled(false)
	m.keymap.settings.SetEnabled(true)
	m.settingsActive = false
	m.active = true
	return m, m.timer.Toggle()
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.progress.Width = msg.Width - 4
		if m.progress.Width > maxWidth {
			m.progress.Width = maxWidth
		}
		return m, nil
	case timer.TickMsg:
		var cmd tea.Cmd

		m.timer, cmd = m.timer.Update(msg)
		m.percent = m.timer.Timeout.Seconds() / m.currentTime.Seconds()
		return m, cmd
	case timer.TimeoutMsg:
		message := "Time to focus!"
		if m.currentTask == "Focus Time" {
			message = "Time for a break"
		}
		err := beeep.Alert(fmt.Sprintf("%s over", m.currentTask), message, "assets/goph.jpg")
		if err != nil {
			panic(err)
		}
		m.keymap.start.SetEnabled(true)
		m.keymap.skip.SetEnabled(false)
	case timer.StartStopMsg:

		var cmd tea.Cmd
		m.timer, cmd = m.timer.Update(msg)
		m.keymap.stop.SetEnabled(m.timer.Running() && !m.settingsActive)
		m.keymap.resume.SetEnabled(!m.timer.Running() && !m.settingsActive)
		m.keymap.start.SetEnabled(m.timer.Timedout() && !m.settingsActive)
		m.keymap.skip.SetEnabled(!m.timer.Timedout() && !m.settingsActive)
		m.keymap.reset.SetEnabled(!m.settingsActive)

		return m, cmd
		// Set focus to next input

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.quit):
			return m, tea.Quit

		}
		if !m.settingsActive {
			switch {
			case key.Matches(msg, m.keymap.start, m.keymap.skip):
				m.active = true
				if m.sessionCounter%2 == 0 {
					m.timer = timer.NewWithInterval(time.Minute*time.Duration(m.focusTime), time.Second)
					m.currentTask = "Focus Time"
				} else {
					m.focusCounter += 1
					if m.focusCounter%3 == 0 {

						m.timer = timer.NewWithInterval(time.Minute*time.Duration(m.longBreak), time.Second)
						m.currentTask = "Long Break Time"

					} else {
						m.timer = timer.NewWithInterval(time.Minute*time.Duration(m.breakTime), time.Second)
						m.currentTask = "Break Time"
					}
				}
				m.currentTime = m.timer.Timeout
				progressBar := progress.New(progress.WithDefaultGradient())
				progressBar.ShowPercentage = false
				m.progress = progressBar
				m.sessionCounter += 1
				return m, m.timer.Start()

			case key.Matches(msg, m.keymap.reset):
				m.timer.Timeout = m.currentTime
			case key.Matches(msg, m.keymap.resume, m.keymap.stop, m.keymap.cancel):
				return m.Resume()
			case key.Matches(msg, m.keymap.settings):
				m.keymap.saveSettings.SetEnabled(true)
				m.keymap.cancel.SetEnabled(true)
				m.keymap.settings.SetEnabled(false)
				m.settingsModel()
				m.settingsActive = true
				m.active = false
				return m, m.timer.Stop()
			}
		} else {
			switch msg.String() {
			case "tab", "shift+tab", "enter", "up", "down":
				if m.settingsActive {
					s := msg.String()

					// Did the user press enter while the submit button was focused?
					if s == "enter" && m.focusIndex == len(m.inputs) {
						errorCount := 0
						newCurrent := 0
						for i, v := range m.inputs {

							n, err := strconv.Atoi(v.Value())
							if err != nil && len(v.Value()) != 0 {
								errorCount++
								m.warningMessage = "input must be a number"
							} else {
								m.warningMessage = ""
								log.Print(n, "num", m.currentTask)
								if n != 0 {
									switch i {
									case 0:
										if m.currentTask == "Focus Time" {
											newCurrent = n
										}
										m.focusTime = n
									case 1:
										if m.currentTask == "Break Time" {
											newCurrent = n
										}
										m.breakTime = n
									case 2:
										if m.currentTask == "Long Break Time" {
											newCurrent = n
										}
										m.longBreak = n
									}
								}
							}
						}
						if errorCount == 0 {
							m.focusIndex = 0
							m.currentTime = time.Minute * time.Duration(newCurrent)
							m.timer.Timeout = m.currentTime
							return m.Resume()
						}

					}

					// Cycle indexes
					if s == "up" || s == "shift+tab" {
						m.focusIndex--
					} else {
						m.focusIndex++
					}

					if m.focusIndex > len(m.inputs) {
						m.focusIndex = 0
					} else if m.focusIndex < 0 {
						m.focusIndex = len(m.inputs)
					}

					cmds := make([]tea.Cmd, len(m.inputs))
					for i := 0; i <= len(m.inputs)-1; i++ {
						if i == m.focusIndex {
							// Set focused state
							cmds[i] = m.inputs[i].Focus()
							m.inputs[i].PromptStyle = focusedStyle
							m.inputs[i].TextStyle = focusedStyle
							continue
						}
						// Remove focused state
						m.inputs[i].Blur()
						m.inputs[i].PromptStyle = noStyle
						m.inputs[i].TextStyle = noStyle
					}
					return m, tea.Batch(cmds...)
				}

			}
		}
	}
	var cmd tea.Cmd
	if m.settingsActive {
		cmd = m.updateInputs(msg)
	}
	return m, cmd
}

func (m model) helpView() string {
	return "\n" + m.help.ShortHelpView([]key.Binding{
		m.keymap.start,
		m.keymap.skip,
		m.keymap.resume,
		m.keymap.stop,
		m.keymap.reset,
		m.keymap.settings,
		m.keymap.saveSettings,
		m.keymap.cancel,
		m.keymap.quit,
	})
}

func (m model) View() string {
	// For a more detailed timer view you could read m.timer.Timeout to get
	// the remaining time as a time.Duration and skip calling m.timer.View()
	// entirely.

	s := m.currentTask + ":" + "\n"

	if m.timer.Timedout() && m.active {
		if m.currentTask == "Focus Time" {
			s = "Well done! Time for a break"
		} else {
			s = "Let's get back to it, time to focus!"
		}
		s += "\n"
	}
	if !m.active {
		s = "Press enter to start\n\n"
	}
	if m.active && !m.timer.Timedout() {
		s += m.progress.ViewAs(m.percent) + " " + m.timer.View() + "\n"
	}
	s += strings.Repeat("•", m.focusCounter)
	if m.settingsActive {
		var b strings.Builder

		s = "Change Settings\n\n"
		for i := range m.inputs {
			b.WriteString(m.inputs[i].View())
			if i < len(m.inputs)-1 {
				b.WriteRune('\n')
			}
		}
		button := &blurredButton
		if m.focusIndex == len(m.inputs) {
			button = &focusedButton
		}
		fmt.Fprintf(&b, "\n\n%s\n\n", *button)
		s += b.String()
	}
	if len(m.warningMessage) != 0 {
		s += "Warning: " + m.warningMessage
	}
	s += m.helpView()
	return s
}
func initModel() model {

	return model{
		breakTime:   5,
		focusTime:   25,
		longBreak:   15,
		percent:     1.0,
		currentTask: "Focus Time",
		keymap: keymap{
			start: key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("[enter]:", "start"),
			),
			skip: key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("[enter]:", "skip"),
			),
			resume: key.NewBinding(
				key.WithKeys(" "),
				key.WithHelp("[space]:", "resume"),
			),
			stop: key.NewBinding(
				key.WithKeys(" "),
				key.WithHelp("[space]:", "pause"),
			),
			reset: key.NewBinding(
				key.WithKeys("r"),
				key.WithHelp("r:", "reset"),
			),
			saveSettings: key.NewBinding(
				key.WithKeys("s", "S"),
				key.WithHelp("s:", "Save settings"),
			),
			cancel: key.NewBinding(
				key.WithKeys("c", "C"),
				key.WithHelp("c:", "Cancel"),
			),
			settings: key.NewBinding(
				key.WithKeys("s", "S"),
				key.WithHelp("s:", "settings"),
			),
			quit: key.NewBinding(
				key.WithKeys("q", "ctrl+c"),
				key.WithHelp("q:", "quit"),
			),
		},
		help: help.New(),
	}
}
