package models

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// pickerChrome is the number of rows View renders around the option list:
// the title, a blank line, a blank line, and the help footer.
const pickerChrome = 4

// minPickerHeight is the smallest option window worth rendering.
const minPickerHeight = 3

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	helpStyle     = lipgloss.NewStyle().Faint(true)
	scrollStyle   = lipgloss.NewStyle().Faint(true)
	filterStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
)

type picker struct {
	all    []Model // every model, unfiltered
	shown  []Model // models matching the current filter
	cursor int     // index into shown
	offset int     // index of the first visible row in shown

	height    int // number of option rows visible at once
	maxHeight int // ceiling derived from the terminal size

	filtering bool
	filter    string

	chosen   *Model
	canceled bool
}

func newPicker(models []Model, maxHeight int) *picker {
	p := &picker{all: models, shown: models, maxHeight: maxHeight}
	p.resize(maxHeight)
	return p
}

// resize recomputes the visible window for a given ceiling.
func (p *picker) resize(maxHeight int) {
	if maxHeight < minPickerHeight {
		maxHeight = minPickerHeight
	}
	p.maxHeight = maxHeight
	p.height = min(len(p.shown), maxHeight)
	if p.height < 1 {
		p.height = 1
	}
	p.clamp()
}

// clamp keeps the cursor in range and scrolls only far enough to keep it
// visible, so rows above the cursor stay on screen.
func (p *picker) clamp() {
	if p.cursor >= len(p.shown) {
		p.cursor = len(p.shown) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+p.height {
		p.offset = p.cursor - p.height + 1
	}
	maxOffset := len(p.shown) - p.height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if p.offset > maxOffset {
		p.offset = maxOffset
	}
	if p.offset < 0 {
		p.offset = 0
	}
}

func (p *picker) applyFilter() {
	if p.filter == "" {
		p.shown = p.all
	} else {
		needle := strings.ToLower(p.filter)
		matches := make([]Model, 0, len(p.all))
		for _, m := range p.all {
			if strings.Contains(strings.ToLower(label(m)), needle) {
				matches = append(matches, m)
			}
		}
		p.shown = matches
	}
	p.cursor = 0
	p.offset = 0
	p.resize(p.maxHeight)
}

func (p *picker) Init() tea.Cmd { return nil }

func (p *picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.resize(msg.Height - pickerChrome)
		return p, nil

	case tea.KeyMsg:
		// While filtering, printable keys edit the filter instead of
		// acting as navigation shortcuts.
		if p.filtering {
			switch msg.Type {
			case tea.KeyRunes, tea.KeySpace:
				p.filter += string(msg.Runes)
				if msg.Type == tea.KeySpace {
					p.filter += " "
				}
				p.applyFilter()
				return p, nil
			case tea.KeyBackspace:
				if p.filter != "" {
					p.filter = p.filter[:len(p.filter)-1]
					p.applyFilter()
				}
				return p, nil
			case tea.KeyEsc:
				p.filtering = false
				p.filter = ""
				p.applyFilter()
				return p, nil
			}
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			p.canceled = true
			return p, tea.Quit
		case "q":
			if !p.filtering {
				p.canceled = true
				return p, tea.Quit
			}
		case "enter":
			if len(p.shown) > 0 {
				chosen := p.shown[p.cursor]
				p.chosen = &chosen
			} else {
				p.canceled = true
			}
			return p, tea.Quit
		case "up", "k":
			p.cursor--
			p.clamp()
		case "down", "j":
			p.cursor++
			p.clamp()
		case "pgup", "b":
			p.cursor -= p.height
			p.clamp()
		case "pgdown", "f":
			p.cursor += p.height
			p.clamp()
		case "home", "g":
			p.cursor = 0
			p.clamp()
		case "end", "G":
			p.cursor = len(p.shown) - 1
			p.clamp()
		case "/":
			if !p.filtering {
				p.filtering = true
				return p, nil
			}
		}
	}
	return p, nil
}

func (p *picker) View() string {
	if p.chosen != nil || p.canceled {
		// Leave no trace: the caller prints the outcome.
		return ""
	}

	var b strings.Builder

	title := fmt.Sprintf("Select a model (%d of %d)", len(p.shown), len(p.all))
	if len(p.shown) == len(p.all) {
		title = fmt.Sprintf("Select a model (%d available)", len(p.all))
	}
	b.WriteString(titleStyle.Render(title))
	if p.filtering {
		b.WriteString("  " + filterStyle.Render("/"+p.filter+"█"))
	}
	b.WriteString("\n\n")

	if len(p.shown) == 0 {
		b.WriteString(helpStyle.Render("  no models match"))
		b.WriteString("\n\n" + helpStyle.Render(p.help()))
		return b.String()
	}

	end := min(p.offset+p.height, len(p.shown))
	for i := p.offset; i < end; i++ {
		line := "  " + label(p.shown[i])
		if i == p.cursor {
			line = cursorStyle.Render("> ") + selectedStyle.Render(label(p.shown[i]))
		}

		// Scroll indicators on the first and last visible rows.
		marker := " "
		if i == p.offset && p.offset > 0 {
			marker = "▲"
		} else if i == end-1 && end < len(p.shown) {
			marker = "▼"
		}
		b.WriteString(line + " " + scrollStyle.Render(marker) + "\n")
	}

	b.WriteString("\n" + helpStyle.Render(p.help()))
	return b.String()
}

func (p *picker) help() string {
	if p.filtering {
		return "type to filter • ↑/↓ navigate • enter select • esc clear filter"
	}
	return "↑/↓ navigate • pgup/pgdn page • / filter • enter select • q cancel"
}
