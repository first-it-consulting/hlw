package cmd

import (
	"errors"
	"slices"
	"testing"

	"github.com/first-it-consulting/hlw/internal/models"
	"github.com/spf13/pflag"
)

// Everything after the harness name belongs to the agent, not to hlw. Without
// SetInterspersed(false) cobra claims those flags itself and
// "hlw launch claude --resume xxxx" fails with "unknown flag: --resume".
func TestLaunchArgsArePassedThroughToTheAgent(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"long flag with value", []string{"claude", "--resume", "xxxx"}, []string{"claude", "--resume", "xxxx"}},
		{"short flag", []string{"claude", "-p", "hello"}, []string{"claude", "-p", "hello"}},
		{"flag hlw itself defines", []string{"claude", "--help"}, []string{"claude", "--help"}},
		{"bare words", []string{"claude", "hello"}, []string{"claude", "hello"}},
		{"double dash", []string{"claude", "--", "-p", "x"}, []string{"claude", "--", "-p", "x"}},
		{"harness only", []string{"claude"}, []string{"claude"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flags := launchCmd.Flags()
			if err := flags.Parse(tc.in); err != nil {
				t.Fatalf("Parse(%v) returned %v; agent flags must not be rejected", tc.in, err)
			}
			if got := flags.Args(); !slices.Equal(got, tc.want) {
				t.Errorf("args = %v, want %v", got, tc.want)
			}
		})
	}
}

// hlw's own flags still work when they come before the harness name. pflag
// signals --help with the ErrHelp sentinel, which cobra turns into usage
// output, so that is the success case here.
func TestLaunchOwnFlagsStillParseBeforeHarnessName(t *testing.T) {
	flags := launchCmd.Flags()
	err := flags.Parse([]string{"--help"})
	if !errors.Is(err, pflag.ErrHelp) {
		t.Fatalf("--help before the harness name should be hlw's own flag, got: %v", err)
	}
	if got := flags.Args(); len(got) != 0 {
		t.Errorf("expected --help to be consumed as a flag, got positional args %v", got)
	}
}

func TestTakeModelArg(t *testing.T) {
	available := []models.Model{
		{ID: "qwen3-coder-30b"},
		{ID: "deepseek-v3.2"},
	}
	cases := []struct {
		name     string
		in       []string
		wantID   string
		wantRest []string
		wantTook bool
	}{
		{"matching model is consumed", []string{"deepseek-v3.2"}, "deepseek-v3.2", []string{}, true},
		{"model then agent flags", []string{"qwen3-coder-30b", "--resume", "x"}, "qwen3-coder-30b", []string{"--resume", "x"}, true},
		// "run" is a real opencode subcommand, not a model.
		{"agent subcommand passes through", []string{"run", "fix the bug"}, "", []string{"run", "fix the bug"}, false},
		{"unknown word passes through", []string{"notamodel"}, "", []string{"notamodel"}, false},
		{"flag is never a model", []string{"--resume", "xxx"}, "", []string{"--resume", "xxx"}, false},
		{"no args", nil, "", nil, false},
		// A model id could start with a dash only by accident; refusing to
		// treat flags as models is the safer rule.
		{"dash-prefixed is left alone", []string{"-m"}, "", []string{"-m"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, rest, took := takeModelArg(available, tc.in)
			if took != tc.wantTook {
				t.Fatalf("took = %v, want %v", took, tc.wantTook)
			}
			if got.ID != tc.wantID {
				t.Errorf("model = %q, want %q", got.ID, tc.wantID)
			}
			if !slices.Equal(rest, tc.wantRest) {
				t.Errorf("rest = %v, want %v", rest, tc.wantRest)
			}
		})
	}
}
