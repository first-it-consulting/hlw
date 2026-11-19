// Package policy detects Claude Code managed (enterprise) settings that would
// silently override what hlw passes to the agent.
//
// Managed settings sit at the top of Claude Code's settings precedence: their
// env block wins over the environment hlw exports, and allowedProviders can
// block a custom endpoint outright. hlw cannot and does not try to override
// them; it only explains why a launch is not going where the config says.
package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// File is one managed settings file and the parts of it hlw cares about.
type File struct {
	Path             string
	Env              map[string]string
	AllowedProviders []string
	// Err is set when the file exists but could not be read or parsed. Its
	// presence is still worth reporting: Claude Code may read it fine.
	Err error
}

type managedSettings struct {
	Env              map[string]string `json:"env"`
	AllowedProviders []string          `json:"allowedProviders"`
}

// searchDirs returns the directories Claude Code reads managed settings from
// on this platform. Tests replace it.
var searchDirs = func() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"/Library/Application Support/ClaudeCode"}
	case "linux":
		return []string{"/etc/claude-code"}
	}
	return nil
}

// FindClaude returns the managed settings files present on this machine: the
// main managed-settings.json and any drop-ins in managed-settings.d/.
// Settings delivered through MDM profiles are not inspected.
func FindClaude() []File {
	var out []File
	for _, dir := range searchDirs() {
		paths := []string{filepath.Join(dir, "managed-settings.json")}
		dropIns, _ := filepath.Glob(filepath.Join(dir, "managed-settings.d", "*.json"))
		slices.Sort(dropIns)
		paths = append(paths, dropIns...)
		for _, p := range paths {
			if f, ok := load(p); ok {
				out = append(out, f)
			}
		}
	}
	return out
}

func load(path string) (File, bool) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return File{}, false
	}
	f := File{Path: path}
	if err != nil {
		f.Err = err
		return f, true
	}
	var s managedSettings
	if err := json.Unmarshal(data, &s); err != nil {
		f.Err = fmt.Errorf("cannot parse: %w", err)
		return f, true
	}
	f.Env = s.Env
	f.AllowedProviders = s.AllowedProviders
	return f, true
}

// providerSwitches are the env vars that route Claude Code to a cloud provider
// instead of ANTHROPIC_BASE_URL.
var providerSwitches = map[string]string{
	"CLAUDE_CODE_USE_BEDROCK": "Amazon Bedrock",
	"CLAUDE_CODE_USE_VERTEX":  "Google Vertex AI",
	"CLAUDE_CODE_USE_FOUNDRY": "Microsoft Foundry",
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "0", "false", "no", "off":
		return false
	}
	return true
}

// Launch describes what hlw is about to hand to Claude Code.
type Launch struct {
	// Endpoint is where the config points the agent ("" if nowhere specific).
	Endpoint string
	// Env is the resolved envVars the harness sets.
	Env map[string]string
	// ModelEnvVars are set to the model picked at launch.
	ModelEnvVars []string
}

// Conflicts lists, per managed file, the ways it overrides this launch. Files
// that change nothing about the launch are left out.
func Conflicts(files []File, l Launch) map[string][]string {
	return conflicts(files, l, false)
}

func conflicts(files []File, l Launch, userSettings bool) map[string][]string {
	out := map[string][]string{}
	add := func(path, msg string) { out[path] = append(out[path], msg) }

	baseURL := strings.TrimRight(l.Env["ANTHROPIC_BASE_URL"], "/")
	pinPath, pin := pinnedBaseURL(files)

	for _, f := range files {
		if f.Err != nil {
			// An unreadable file is worth reporting either way: Claude Code
			// might read it fine (MDM or root-only permissions), and staying
			// silent would falsely claim the launch is unoverridden.
			what := "managed settings"
			if userSettings {
				what = "your user settings"
			}
			add(f.Path, fmt.Sprintf("present but unreadable by hlw (%v); these %s may still override this launch", f.Err, what))
		}
		for _, k := range sortedKeys(f.Env) {
			v := f.Env[k]
			if name, ok := providerSwitches[k]; ok && truthy(v) {
				if truthy(l.Env[k]) {
					continue // the harness wants this provider too
				}
				target := "the configured endpoint"
				if l.Endpoint != "" {
					target = l.Endpoint
				}
				add(f.Path, fmt.Sprintf("sets %s=%s: Claude Code will use %s, not %s", k, v, name, target))
				continue
			}
			if k == "ANTHROPIC_BASE_URL" {
				continue // judged together with allowedProviders below
			}
			if slices.Contains(l.ModelEnvVars, k) {
				add(f.Path, fmt.Sprintf("sets %s=%s: the model you pick is ignored", k, v))
				continue
			}
			if hv, ok := l.Env[k]; ok && hv != v {
				add(f.Path, fmt.Sprintf("sets %s=%s, overriding hlw's %s", k, v, hv))
			}
		}
	}

	if baseURL == "" {
		return out
	}
	if pin != "" && pin != baseURL {
		add(pinPath, fmt.Sprintf("pins ANTHROPIC_BASE_URL=%s: Claude Code goes there, not to %s", pin, baseURL))
	}
	if listPath, allowed, ok := allowedProviders(files); ok {
		switch {
		case !slices.Contains(allowed, "customEndpoint"):
			add(listPath, fmt.Sprintf("allowedProviders = [%s] does not include \"customEndpoint\": Claude Code refuses ANTHROPIC_BASE_URL=%s", strings.Join(allowed, ", "), baseURL))
		case pin == "":
			add(listPath, fmt.Sprintf("allowedProviders includes \"customEndpoint\", but no managed env pins ANTHROPIC_BASE_URL: Claude Code refuses %s until the policy pins it", baseURL))
		}
	}
	return out
}

// allowedProviders returns the managed provider list and the file it comes
// from. Claude Code loads managed-settings.json first and merges the
// managed-settings.d drop-ins on top in name order, and the highest source that
// sets the list owns it whole: the last drop-in with a list wins, then the main
// file.
func allowedProviders(files []File) (string, []string, bool) {
	for i := len(files) - 1; i >= 0; i-- {
		if files[i].AllowedProviders != nil && !isMain(files[i].Path) {
			return files[i].Path, files[i].AllowedProviders, true
		}
	}
	for _, f := range files {
		if f.AllowedProviders != nil && isMain(f.Path) {
			return f.Path, f.AllowedProviders, true
		}
	}
	return "", nil, false
}

// pinnedBaseURL returns the ANTHROPIC_BASE_URL a managed file pins, if any.
// Like the env block itself, the pin is a deep merge across files: the last
// file that sets it wins, drop-ins included. This is deliberately not
// allowedProviders' whole-list-wins rule below.
func pinnedBaseURL(files []File) (string, string) {
	path, pin := "", ""
	for _, f := range files {
		if v := f.Env["ANTHROPIC_BASE_URL"]; v != "" {
			path, pin = f.Path, strings.TrimRight(v, "/")
		}
	}
	return path, pin
}

func isMain(path string) bool {
	return filepath.Base(path) == "managed-settings.json"
}

// userSettingsPath is Claude Code's user settings file. Tests replace it.
var userSettingsPath = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "settings.json")
}

// UserSettingsConflicts reports env entries in ~/.claude/settings.json that
// override what hlw exports. Those can be beaten by passing the env through
// --settings (the envViaSettings harness option); managed ones cannot.
// allowedProviders is ignored there: it only applies to managed settings.
func UserSettingsConflicts(l Launch) map[string][]string {
	p := userSettingsPath()
	if p == "" {
		return nil
	}
	f, ok := load(p)
	if !ok {
		return nil
	}
	f.AllowedProviders = nil // only meaningful in managed settings
	return conflicts([]File{f}, l, true)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// IsClaude reports whether the executable is Claude Code, the only harness
// these managed settings apply to.
func IsClaude(executable string) bool {
	return filepath.Base(executable) == "claude"
}
