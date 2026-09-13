package models

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func testModels(n int) []Model {
	out := make([]Model, n)
	for i := range out {
		out[i] = Model{ID: string(rune('a' + i))}
	}
	return out
}

func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "home":
		return tea.KeyMsg{Type: tea.KeyHome}
	case "end":
		return tea.KeyMsg{Type: tea.KeyEnd}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func press(p *picker, keys ...string) {
	for _, k := range keys {
		p.Update(key(k))
	}
}

// The original huh-based picker reset the scroll offset to the cursor on every
// keystroke, so the selection was always pinned to the top row and rows above
// it were unreachable. Moving down within the window must not scroll at all.
func TestPicker_CursorMovesWithinWindowBeforeScrolling(t *testing.T) {
	p := newPicker(testModels(20), 10)

	for i := 1; i < 10; i++ {
		press(p, "down")
		if p.offset != 0 {
			t.Fatalf("after %d downs: offset moved to %d, expected the window to stay put", i, p.offset)
		}
		if p.cursor != i {
			t.Fatalf("after %d downs: cursor = %d, want %d", i, p.cursor, i)
		}
	}

	// The 10th press pushes past the bottom row, so now it should scroll by one.
	press(p, "down")
	if p.cursor != 10 {
		t.Errorf("cursor = %d, want 10", p.cursor)
	}
	if p.offset != 1 {
		t.Errorf("offset = %d, want 1 (scroll by exactly one row)", p.offset)
	}
}

// The cursor must always be inside the visible window.
func TestPicker_CursorAlwaysVisible(t *testing.T) {
	p := newPicker(testModels(30), 7)
	for i := 0; i < 60; i++ {
		press(p, "down")
		if p.cursor < p.offset || p.cursor >= p.offset+p.height {
			t.Fatalf("step %d: cursor %d outside window [%d,%d)", i, p.cursor, p.offset, p.offset+p.height)
		}
	}
	for i := 0; i < 60; i++ {
		press(p, "up")
		if p.cursor < p.offset || p.cursor >= p.offset+p.height {
			t.Fatalf("up step %d: cursor %d outside window [%d,%d)", i, p.cursor, p.offset, p.offset+p.height)
		}
	}
}

func TestPicker_ScrollingUpKeepsRowsBelowVisible(t *testing.T) {
	p := newPicker(testModels(20), 5)
	press(p, "end") // jump to the bottom
	if p.cursor != 19 {
		t.Fatalf("cursor = %d, want 19", p.cursor)
	}
	if p.offset != 15 {
		t.Fatalf("offset = %d, want 15", p.offset)
	}

	// Moving up inside the window must not move the window.
	for i := 0; i < 4; i++ {
		press(p, "up")
		if p.offset != 15 {
			t.Fatalf("after %d ups: offset = %d, want it to stay at 15", i+1, p.offset)
		}
	}
	press(p, "up")
	if p.offset != 14 {
		t.Errorf("offset = %d, want 14", p.offset)
	}
}

func TestPicker_OffsetNeverExceedsBounds(t *testing.T) {
	p := newPicker(testModels(12), 5)
	press(p, "end", "down", "down", "down")
	if p.offset+p.height > len(p.shown) {
		t.Errorf("window [%d,%d) runs past %d models", p.offset, p.offset+p.height, len(p.shown))
	}
	if p.offset < 0 {
		t.Errorf("negative offset %d", p.offset)
	}
}

func TestPicker_HomeAndEnd(t *testing.T) {
	p := newPicker(testModels(20), 6)
	press(p, "end")
	if p.cursor != 19 {
		t.Errorf("end: cursor = %d, want 19", p.cursor)
	}
	press(p, "home")
	if p.cursor != 0 || p.offset != 0 {
		t.Errorf("home: cursor = %d, offset = %d, want 0/0", p.cursor, p.offset)
	}
}

func TestPicker_PageKeys(t *testing.T) {
	p := newPicker(testModels(40), 10)
	press(p, "pgdown")
	if p.cursor != 10 {
		t.Errorf("pgdown: cursor = %d, want 10", p.cursor)
	}
	press(p, "pgup")
	if p.cursor != 0 {
		t.Errorf("pgup: cursor = %d, want 0", p.cursor)
	}
}

func TestPicker_FitsSmallLists(t *testing.T) {
	p := newPicker(testModels(3), 20)
	if p.height != 3 {
		t.Errorf("height = %d, want 3 (no padding beyond the list)", p.height)
	}
	press(p, "down", "down", "down", "down")
	if p.cursor != 2 {
		t.Errorf("cursor = %d, want it clamped to 2", p.cursor)
	}
	if p.offset != 0 {
		t.Errorf("offset = %d, want 0 for a list that fits", p.offset)
	}
}

func TestPicker_EnterSelectsCursor(t *testing.T) {
	p := newPicker(testModels(5), 5)
	press(p, "down", "down", "enter")
	if p.chosen == nil || p.chosen.ID != "c" {
		t.Errorf("chosen = %+v, want c", p.chosen)
	}
	if p.canceled {
		t.Error("enter should not cancel")
	}
}

func TestPicker_CancelKeys(t *testing.T) {
	for _, k := range []string{"q", "esc", "ctrl+c"} {
		t.Run(k, func(t *testing.T) {
			p := newPicker(testModels(5), 5)
			if k == "ctrl+c" {
				p.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
			} else {
				press(p, k)
			}
			if !p.canceled {
				t.Errorf("%s should cancel", k)
			}
			if p.chosen != nil {
				t.Errorf("%s should not choose anything, got %+v", k, p.chosen)
			}
		})
	}
}

func TestPicker_FilterNarrowsAndResets(t *testing.T) {
	list := []Model{{ID: "alpha"}, {ID: "beta"}, {ID: "gamma"}, {ID: "alphabet"}}
	p := newPicker(list, 10)

	press(p, "/", "a", "l", "p", "h")
	if len(p.shown) != 2 {
		t.Fatalf("filter \"alph\" matched %d, want 2: %+v", len(p.shown), p.shown)
	}
	if p.height != 2 {
		t.Errorf("height = %d, want it to shrink to 2", p.height)
	}

	press(p, "esc")
	if len(p.shown) != 4 || p.filtering {
		t.Errorf("esc should clear the filter, got %d shown, filtering=%v", len(p.shown), p.filtering)
	}
}

// While filtering, keys that are navigation shortcuts must type instead.
func TestPicker_FilterSwallowsNavigationRunes(t *testing.T) {
	list := []Model{{ID: "jkq-one"}, {ID: "other"}}
	p := newPicker(list, 10)
	press(p, "/", "j", "k", "q")
	if p.filter != "jkq" {
		t.Errorf("filter = %q, want \"jkq\"", p.filter)
	}
	if p.canceled {
		t.Error("q must not cancel while filtering")
	}
	if len(p.shown) != 1 || p.shown[0].ID != "jkq-one" {
		t.Errorf("expected the jkq-one match, got %+v", p.shown)
	}
}

func TestPicker_FilterWithNoMatchesDoesNotPanic(t *testing.T) {
	p := newPicker(testModels(5), 5)
	press(p, "/", "z", "z", "z")
	if len(p.shown) != 0 {
		t.Fatalf("expected no matches, got %d", len(p.shown))
	}
	press(p, "down", "up", "end", "home")
	view := p.View()
	if !strings.Contains(view, "no models match") {
		t.Errorf("expected an empty-state message, got:\n%s", view)
	}
	press(p, "enter")
	if !p.canceled {
		t.Error("enter with no matches should cancel rather than select nothing")
	}
}

func TestPicker_ResizeKeepsCursorVisible(t *testing.T) {
	p := newPicker(testModels(30), 20)
	press(p, "end")
	p.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	if p.cursor < p.offset || p.cursor >= p.offset+p.height {
		t.Errorf("after resize, cursor %d outside window [%d,%d)", p.cursor, p.offset, p.offset+p.height)
	}
	if p.height > 10-pickerChrome {
		t.Errorf("height %d exceeds the space a 10-row terminal leaves", p.height)
	}
}

func TestPicker_ViewShowsScrollIndicators(t *testing.T) {
	p := newPicker(testModels(20), 5)
	if v := p.View(); !strings.Contains(v, "▼") || strings.Contains(v, "▲") {
		t.Errorf("at the top expected ▼ only, got:\n%s", v)
	}
	press(p, "end")
	if v := p.View(); !strings.Contains(v, "▲") || strings.Contains(v, "▼") {
		t.Errorf("at the bottom expected ▲ only, got:\n%s", v)
	}
}
