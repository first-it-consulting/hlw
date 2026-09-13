package models

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// withStdin runs fn with os.Stdin replaced by a pipe carrying input. Because
// the pipe is not a TTY, SelectModel takes the numbered fallback path.
func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()
	orig := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = r
	defer func() {
		os.Stdin = orig
		r.Close()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.WriteString(input)
		w.Close()
	}()
	fn()
	<-done
}

func TestSelectModel_NoModels(t *testing.T) {
	if _, err := SelectModel(nil); err == nil {
		t.Fatal("Expected error when there are no models")
	}
}

func TestSelectModel_SingleModelSkipsPrompt(t *testing.T) {
	// No stdin is provided: a single model must not prompt at all.
	got, err := SelectModel([]Model{{ID: "only-model"}})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if got.ID != "only-model" {
		t.Errorf("Expected 'only-model', got: %s", got.ID)
	}
}

func TestSelectModel_FallbackPicksByNumber(t *testing.T) {
	list := []Model{{ID: "model-1"}, {ID: "model-2"}, {ID: "model-3"}}
	var got Model
	var err error
	withStdin(t, "3\n", func() { got, err = SelectModel(list) })
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if got.ID != "model-3" {
		t.Errorf("Expected 'model-3', got: %s", got.ID)
	}
}

// The old implementation aborted the launch on the first bad entry; it should
// re-prompt instead.
func TestSelectModel_FallbackRepromptsOnBadInput(t *testing.T) {
	list := []Model{{ID: "model-1"}, {ID: "model-2"}}
	var got Model
	var err error
	withStdin(t, "abc\n99\n0\n2\n", func() { got, err = SelectModel(list) })
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if got.ID != "model-2" {
		t.Errorf("Expected 'model-2', got: %s", got.ID)
	}
}

func TestSelectModel_FallbackGivesUpOnEOF(t *testing.T) {
	list := []Model{{ID: "model-1"}, {ID: "model-2"}}
	var err error
	withStdin(t, "", func() { _, err = SelectModel(list) })
	if err == nil {
		t.Fatal("Expected an error when stdin is exhausted")
	}
	if errors.Is(err, ErrSelectionCanceled) {
		t.Error("EOF should not be reported as a user cancellation")
	}
}

func TestLabel(t *testing.T) {
	cases := []struct {
		name string
		in   Model
		want string
	}{
		{"id only", Model{ID: "m1"}, "m1"},
		{"display name", Model{ID: "m1", DisplayName: "Model One"}, "m1 (Model One)"},
		{"name fallback", Model{ID: "m1", Name: "Model One"}, "m1 (Model One)"},
		{"display name equal to id", Model{ID: "m1", DisplayName: "m1"}, "m1"},
		{"display name wins over name", Model{ID: "m1", Name: "n", DisplayName: "d"}, "m1 (d)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := label(tc.in); got != tc.want {
				t.Errorf("label(%+v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSelectModel_FallbackListsEveryModel(t *testing.T) {
	list := []Model{{ID: "alpha"}, {ID: "beta"}, {ID: "gamma"}}

	origOut := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var out strings.Builder
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			out.Write(buf[:n])
			if err != nil {
				return
			}
		}
	}()

	withStdin(t, "1\n", func() { SelectModel(list) })

	w.Close()
	os.Stdout = origOut
	<-done

	for _, m := range list {
		if !strings.Contains(out.String(), m.ID) {
			t.Errorf("Expected %q in the prompt output, got:\n%s", m.ID, out.String())
		}
	}
}

func TestModel_ContextWindow(t *testing.T) {
	cases := []struct {
		name string
		in   Model
		want int
	}{
		{"max_model_len", Model{MaxModelLen: 262144}, 262144},
		{"context_length", Model{ContextLength: 128000}, 128000},
		{"max_model_len wins", Model{MaxModelLen: 262144, ContextLength: 8192}, 262144},
		{"unknown", Model{}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.ContextWindow(); got != tc.want {
				t.Errorf("ContextWindow() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestSelectModel_CarriesMetadataThrough(t *testing.T) {
	list := []Model{{ID: "a"}, {ID: "b", MaxModelLen: 262144}}
	var got Model
	withStdin(t, "2\n", func() { got, _ = SelectModel(list) })
	if got.ID != "b" || got.ContextWindow() != 262144 {
		t.Errorf("got %+v, want b with a 262144 context window", got)
	}
}
