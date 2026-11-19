package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	orig := searchDirs
	searchDirs = func() []string { return []string{dir} }
	t.Cleanup(func() { searchDirs = orig })
	return dir
}

func TestFindClaudeNoPolicy(t *testing.T) {
	withDir(t, nil)
	if got := FindClaude(); len(got) != 0 {
		t.Fatalf("expected no files, got %+v", got)
	}
}

func TestFindClaudeReadsMainAndDropIns(t *testing.T) {
	withDir(t, map[string]string{
		"managed-settings.json":          `{"allowedProviders":["bedrock"],"env":{"CLAUDE_CODE_USE_BEDROCK":"1"}}`,
		"managed-settings.d/10-x.json":   `{"env":{"ANTHROPIC_MODEL":"m"}}`,
		"managed-settings.d/20-bad.json": `{not json`,
	})
	got := FindClaude()
	if len(got) != 3 {
		t.Fatalf("expected 3 files, got %d", len(got))
	}
	if got[0].Env["CLAUDE_CODE_USE_BEDROCK"] != "1" || got[0].AllowedProviders[0] != "bedrock" {
		t.Errorf("main file not parsed: %+v", got[0])
	}
	if got[2].Err == nil {
		t.Errorf("expected parse error for bad drop-in")
	}
}

func TestConflictsBedrockPolicy(t *testing.T) {
	files := []File{{
		Path:             "/managed.json",
		AllowedProviders: []string{"bedrock"},
		Env: map[string]string{
			"CLAUDE_CODE_USE_BEDROCK":        "1",
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "eu.anthropic.claude-sonnet-5-5",
			"API_TIMEOUT_MS":                 "600000",
			"AWS_REGION":                     "eu-central-1",
		},
	}}
	l := Launch{
		Endpoint:     "http://127.0.0.1:4000",
		Env:          map[string]string{"ANTHROPIC_BASE_URL": "http://127.0.0.1:4000", "API_TIMEOUT_MS": "3000000"},
		ModelEnvVars: []string{"ANTHROPIC_DEFAULT_SONNET_MODEL"},
	}
	msgs := Conflicts(files, l)["/managed.json"]
	joined := strings.Join(msgs, "\n")
	for _, want := range []string{
		"Amazon Bedrock, not http://127.0.0.1:4000",
		"ANTHROPIC_DEFAULT_SONNET_MODEL=eu.anthropic.claude-sonnet-5-5: the model you pick is ignored",
		"API_TIMEOUT_MS=600000, overriding hlw's 3000000",
		"allowedProviders = [bedrock]",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "AWS_REGION") {
		t.Errorf("unrelated policy var should not be reported:\n%s", joined)
	}
}

func TestConflictsNoneWhenPolicyIsHarmless(t *testing.T) {
	files := []File{{Path: "/m.json", Env: map[string]string{"DISABLE_TELEMETRY": "1", "CLAUDE_CODE_USE_BEDROCK": "0"}}}
	l := Launch{Env: map[string]string{"ANTHROPIC_BASE_URL": "http://x"}}
	if got := Conflicts(files, l); len(got) != 0 {
		t.Fatalf("expected no conflicts, got %v", got)
	}
}

func TestConflictsHarnessAlsoWantsBedrock(t *testing.T) {
	files := []File{{Path: "/m.json", Env: map[string]string{"CLAUDE_CODE_USE_BEDROCK": "1"}}}
	l := Launch{Env: map[string]string{"CLAUDE_CODE_USE_BEDROCK": "1"}}
	if got := Conflicts(files, l); len(got) != 0 {
		t.Fatalf("expected no conflicts, got %v", got)
	}
}

func TestIsClaude(t *testing.T) {
	for exe, want := range map[string]bool{"claude": true, "/usr/local/bin/claude": true, "codex": false, "claude-foo": false} {
		if IsClaude(exe) != want {
			t.Errorf("IsClaude(%q) = %v", exe, !want)
		}
	}
}

func TestConflictsCustomEndpoint(t *testing.T) {
	l := Launch{
		Endpoint: "http://127.0.0.1:4000",
		Env:      map[string]string{"ANTHROPIC_BASE_URL": "http://127.0.0.1:4000/", "CLAUDE_CODE_USE_BEDROCK": "0"},
	}
	cases := []struct {
		name string
		file File
		want string // "" = no conflict expected
	}{
		{
			name: "listed and pinned to the same URL",
			file: File{Path: "/m/managed-settings.json", AllowedProviders: []string{"bedrock", "customEndpoint"}, Env: map[string]string{"ANTHROPIC_BASE_URL": "http://127.0.0.1:4000"}},
		},
		{
			name: "listed but not pinned",
			file: File{Path: "/m/managed-settings.json", AllowedProviders: []string{"bedrock", "customEndpoint"}},
			want: "no managed env pins ANTHROPIC_BASE_URL",
		},
		{
			name: "pinned to another URL",
			file: File{Path: "/m/managed-settings.json", AllowedProviders: []string{"customEndpoint"}, Env: map[string]string{"ANTHROPIC_BASE_URL": "https://gw.example"}},
			want: "pins ANTHROPIC_BASE_URL=https://gw.example: Claude Code goes there, not to http://127.0.0.1:4000",
		},
		{
			name: "not listed",
			file: File{Path: "/m/managed-settings.json", AllowedProviders: []string{"bedrock"}},
			want: `does not include "customEndpoint"`,
		},
		{
			name: "no list, no pin",
			file: File{Path: "/m/managed-settings.json", Env: map[string]string{"DISABLE_TELEMETRY": "1"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Conflicts([]File{tc.file}, l)
			joined := strings.Join(got[tc.file.Path], "\n")
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("expected no conflicts, got %v", got)
				}
				return
			}
			if !strings.Contains(joined, tc.want) {
				t.Errorf("want %q in:\n%s", tc.want, joined)
			}
		})
	}
}

func TestPinnedBaseURLMergePrecedence(t *testing.T) {
	// The env block merges key by key, so the last file that sets a key owns
	// it — unlike allowedProviders, where only a drop-in can beat the main
	// file. A main-file pin must lose to a drop-in pin.
	files := []File{
		{Path: "/m/managed-settings.json", Env: map[string]string{"ANTHROPIC_BASE_URL": "https://main.example"}},
		{Path: "/m/managed-settings.d/50-pilot.json", Env: map[string]string{"ANTHROPIC_BASE_URL": "https://pilot.example"}},
	}
	l := Launch{Env: map[string]string{"ANTHROPIC_BASE_URL": "https://pilot.example"}}
	if got := Conflicts(files, l); len(got) != 0 {
		t.Errorf("drop-in pin should win and match the launch, got %v", got)
	}
	// And be reported against, attributed to the file that owns the pin.
	l.Env["ANTHROPIC_BASE_URL"] = "https://other.example"
	joined := strings.Join(Conflicts(files, l)["/m/managed-settings.d/50-pilot.json"], "\n")
	if !strings.Contains(joined, "pins ANTHROPIC_BASE_URL=https://pilot.example") {
		t.Errorf("want the drop-in's pin reported, got:\n%s", joined)
	}
}

func TestConflictsDropInListWinsOverMain(t *testing.T) {
	// Drop-ins merge on top of managed-settings.json, so a pilot drop-in can
	// widen the list without touching the main file.
	files := []File{
		{Path: "/m/managed-settings.json", AllowedProviders: []string{"bedrock"}, Env: map[string]string{"ANTHROPIC_BASE_URL": "http://x"}},
		{Path: "/m/managed-settings.d/50-pilot.json", AllowedProviders: []string{"bedrock", "customEndpoint"}},
	}
	l := Launch{Env: map[string]string{"ANTHROPIC_BASE_URL": "http://x"}}
	if got := Conflicts(files, l); len(got) != 0 {
		t.Errorf("drop-in list should win and admit the pinned endpoint, got %v", got)
	}
	// And the other way round: a drop-in can narrow it again.
	files[1].AllowedProviders = []string{"bedrock"}
	files[0].AllowedProviders = []string{"bedrock", "customEndpoint"}
	got := Conflicts(files, l)
	if !strings.Contains(strings.Join(got["/m/managed-settings.d/50-pilot.json"], "\n"), `does not include "customEndpoint"`) {
		t.Errorf("expected the drop-in's list to win, got %v", got)
	}
}

func TestUserSettingsConflicts(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	os.WriteFile(p, []byte(`{"allowedProviders":["bedrock"],"env":{"CLAUDE_CODE_USE_BEDROCK":"1","ANTHROPIC_DEFAULT_OPUS_MODEL":"eu.opus"}}`), 0o644)
	orig := userSettingsPath
	userSettingsPath = func() string { return p }
	t.Cleanup(func() { userSettingsPath = orig })

	l := Launch{
		Endpoint:     "http://127.0.0.1:4000",
		Env:          map[string]string{"ANTHROPIC_BASE_URL": "http://127.0.0.1:4000"},
		ModelEnvVars: []string{"ANTHROPIC_DEFAULT_OPUS_MODEL"},
	}
	joined := strings.Join(UserSettingsConflicts(l)[p], "\n")
	for _, want := range []string{"Amazon Bedrock", "the model you pick is ignored"} {
		if !strings.Contains(joined, want) {
			t.Errorf("want %q in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "allowedProviders") {
		t.Errorf("allowedProviders in user settings must be ignored:\n%s", joined)
	}

	userSettingsPath = func() string { return filepath.Join(dir, "missing.json") }
	if got := UserSettingsConflicts(l); len(got) != 0 {
		t.Errorf("missing file should give no conflicts, got %v", got)
	}

	// An unparseable file must not silently report "no conflicts": staying
	// silent would falsely claim the launch is unoverridden.
	os.WriteFile(p, []byte(`{not json`), 0o644)
	userSettingsPath = func() string { return p }
	joined = strings.Join(UserSettingsConflicts(l)[p], "\n")
	if !strings.Contains(joined, "unreadable by hlw") || !strings.Contains(joined, "your user settings") {
		t.Errorf("want an unreadable-user-settings notice, got:\n%s", joined)
	}
}
