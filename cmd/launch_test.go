package cmd

import (
	"errors"
	"slices"
	"testing"

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
