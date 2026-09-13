package models

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

// ErrSelectionCanceled is returned when the user aborts the model picker.
var ErrSelectionCanceled = errors.New("model selection canceled")

// maxPickerHeight caps how many models are shown at once in the interactive
// picker; the list scrolls beyond that.
const maxPickerHeight = 15

// SelectModel asks the user to choose a model.
//
// On an interactive terminal this shows a scrolling arrow-key picker. When
// stdin or stdout is not a TTY (piped input, CI, a dumb terminal), or the
// picker fails to start, it falls back to a numbered prompt.
func SelectModel(models []Model) (Model, error) {
	if len(models) == 0 {
		return Model{}, fmt.Errorf("no models available to select from")
	}

	if len(models) == 1 {
		fmt.Printf("Using only available model: %s\n", models[0].ID)
		return models[0], nil
	}

	if isInteractive() {
		m, err := selectInteractive(models)
		if err == nil {
			return m, nil
		}
		if errors.Is(err, ErrSelectionCanceled) {
			return Model{}, err
		}
		// Picker could not run (no terminfo, unsupported terminal, ...).
		// Fall through to the numbered prompt rather than failing the launch.
	}

	return selectNumbered(models)
}

func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// selectInteractive shows the scrolling picker.
func selectInteractive(models []Model) (Model, error) {
	p := newPicker(models, maxPickerHeight)

	final, err := tea.NewProgram(p).Run()
	if err != nil {
		return Model{}, err
	}

	result, ok := final.(*picker)
	if !ok {
		return Model{}, fmt.Errorf("unexpected picker state")
	}
	if result.canceled || result.chosen == nil {
		return Model{}, ErrSelectionCanceled
	}
	return *result.chosen, nil
}

// selectNumbered is the non-TTY fallback: print a numbered list and read a
// number from stdin, re-prompting until the input is valid.
func selectNumbered(models []Model) (Model, error) {
	fmt.Printf("\nSelect a model (%d available):\n", len(models))
	for i, m := range models {
		fmt.Printf("  %d. %s\n", i+1, label(m))
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("\nEnter model number [1-%d]: ", len(models))
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			// EOF with nothing typed: nothing more is coming.
			return Model{}, fmt.Errorf("failed to read selection: %w", err)
		}

		choice, convErr := strconv.Atoi(strings.TrimSpace(line))
		if convErr == nil && choice >= 1 && choice <= len(models) {
			fmt.Println()
			return models[choice-1], nil
		}

		fmt.Printf("Please enter a number between 1 and %d.\n", len(models))
		if err != nil {
			// Input stream is exhausted; stop rather than loop forever.
			return Model{}, fmt.Errorf("failed to read selection: %w", err)
		}
	}
}

// label renders a model as "id (display name)", omitting a display name that
// is absent or identical to the id.
func label(m Model) string {
	name := m.DisplayName
	if name == "" {
		name = m.Name
	}
	if name == "" || name == m.ID {
		return m.ID
	}
	return fmt.Sprintf("%s (%s)", m.ID, name)
}
