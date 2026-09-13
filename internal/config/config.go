package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var testConfigPath string

type HarnessConfig struct {
	Executable string            `json:"executable"`
	Args       []string          `json:"args,omitempty"`
	EnvVars    map[string]string `json:"envVars,omitempty"`

	// Endpoint is the base URL of an OpenAI-compatible server. The model list
	// is fetched from <endpoint>/v1/models. This is the normal way to point a
	// harness at a server.
	Endpoint string `json:"endpoint,omitempty"`

	// ModelURL is the full URL of the model list, for servers that do not put
	// it at /v1/models. It overrides Endpoint when both are set.
	ModelURL string `json:"modelURL,omitempty"`

	// ModelEnvVars names environment variables that are set to the selected
	// model id. Claude Code takes its model this way.
	ModelEnvVars []string `json:"modelEnvVars,omitempty"`

	// ModelArgs are appended after Args, with every occurrence of
	// ModelPlaceholder replaced by the selected model id. Codex, OpenCode and
	// Pi all take their model as a command-line flag rather than an env var.
	ModelArgs []string `json:"modelArgs,omitempty"`

	// ModelAuthToken is the bearer token sent when fetching the model list.
	// It supports the same placeholders as EnvVars values, so the secret can
	// come from the environment with {{env:NAME}} rather than being stored
	// here. Without it the model list is fetched unauthenticated: there is no
	// implicit token lookup.
	ModelAuthToken string `json:"modelAuthToken,omitempty"`

	Description string `json:"description,omitempty"`
}

// Placeholder names usable inside ModelArgs, written as {{name}}.
const (
	// PlaceholderModel is the selected model's id.
	PlaceholderModel = "model"
	// PlaceholderContextWindow is the model's context window in tokens, when
	// the endpoint reports one.
	PlaceholderContextWindow = "contextWindow"
	// PlaceholderEndpoint is the harness's endpoint, usable in envVars values
	// so a base URL need not be written twice.
	PlaceholderEndpoint = "endpoint"
)

// ModelPlaceholder is the {{model}} token, kept for convenience.
const ModelPlaceholder = "{{" + PlaceholderModel + "}}"

// knownPlaceholders is the set accepted in ModelArgs.
var knownPlaceholders = map[string]bool{
	PlaceholderModel:         true,
	PlaceholderContextWindow: true,
	PlaceholderEndpoint:      true,
}

// envPlaceholders are the ones available in envVars values. The model is not
// among them: envVars are resolved before a model has been chosen, and
// modelEnvVars already covers that case.
var envPlaceholders = map[string]bool{
	PlaceholderEndpoint: true,
}

// placeholderPattern matches a {{name}} token, including the {{env:NAME}} form.
var placeholderPattern = regexp.MustCompile(`\{\{([A-Za-z][A-Za-z0-9_]*(?::[A-Za-z_][A-Za-z0-9_]*)?)\}\}`)

// envPlaceholderPrefix marks a placeholder whose value comes from the process
// environment: {{env:OMLX_API_KEY}}.
const envPlaceholderPrefix = "env:"

// mistypedPlaceholder catches ((name)), a plausible guess at the syntax that
// would otherwise be exported verbatim.
var mistypedPlaceholder = regexp.MustCompile(`\(\([A-Za-z][A-Za-z0-9_:]*\)\)`)

type RootConfig struct {
	Harnesses map[string]HarnessConfig `json:"harnesses"`
}

// DefaultModelsPath is appended to Endpoint to reach the model list.
const DefaultModelsPath = "/v1/models"

const DefaultConfigDir = ".config/hlw"
const DefaultConfigFile = "config.json"

func LoadConfig() (*RootConfig, error) {
	configPath := ConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}
	var config RootConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}
	if err := ValidateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &config, nil
}

func ValidateConfig(config *RootConfig) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}
	if len(config.Harnesses) == 0 {
		return fmt.Errorf("no harnesses configured")
	}
	// Iterate in sorted order so a config with several problems always
	// reports the same one first.
	for _, name := range sortedKeys(config.Harnesses) {
		harness := config.Harnesses[name]
		if err := validateHarness(name, &harness); err != nil {
			return err
		}
	}
	return nil
}

func validateHarness(name string, harness *HarnessConfig) error {
	if harness.Executable == "" {
		return fmt.Errorf("harness %q: executable is required", name)
	}
	// Model wiring is only reachable when there is a model list to pick from.
	if harness.ModelListURL() == "" {
		if len(harness.ModelEnvVars) > 0 {
			return fmt.Errorf("harness %q: modelEnvVars needs endpoint or modelURL to be set", name)
		}
		if len(harness.ModelArgs) > 0 {
			return fmt.Errorf("harness %q: modelArgs needs endpoint or modelURL to be set", name)
		}
	}
	for _, arg := range harness.ModelArgs {
		for _, match := range placeholderPattern.FindAllStringSubmatch(arg, -1) {
			if !knownPlaceholders[match[1]] {
				return fmt.Errorf("harness %q: unknown placeholder %q in modelArgs (known: {{model}}, {{contextWindow}}, {{endpoint}})", name, match[0])
			}
		}
	}
	if m := mistypedPlaceholder.FindString(harness.ModelAuthToken); m != "" {
		return fmt.Errorf("harness %q: modelAuthToken contains %s; placeholders are written {{name}}, not ((name))", name, m)
	}
	for _, match := range placeholderPattern.FindAllStringSubmatch(harness.ModelAuthToken, -1) {
		if strings.HasPrefix(match[1], envPlaceholderPrefix) {
			continue
		}
		if !envPlaceholders[match[1]] {
			return fmt.Errorf("harness %q: placeholder %q is not available in modelAuthToken (known: {{endpoint}}, {{env:NAME}})", name, match[0])
		}
		if harness.Endpoint == "" {
			return fmt.Errorf("harness %q: modelAuthToken uses {{endpoint}} but no endpoint is set", name)
		}
	}
	for _, key := range harness.SortedEnvVars() {
		if m := mistypedPlaceholder.FindString(harness.EnvVars[key]); m != "" {
			return fmt.Errorf("harness %q: envVars %q contains %s; placeholders are written {{name}}, not ((name))", name, key, m)
		}
		for _, match := range placeholderPattern.FindAllStringSubmatch(harness.EnvVars[key], -1) {
			if strings.HasPrefix(match[1], envPlaceholderPrefix) {
				continue
			}
			if !envPlaceholders[match[1]] {
				return fmt.Errorf("harness %q: placeholder %q is not available in envVars %q (known: {{endpoint}}, {{env:NAME}})", name, match[0], key)
			}
			if harness.Endpoint == "" {
				return fmt.Errorf("harness %q: envVars %q uses {{endpoint}} but no endpoint is set", name, key)
			}
		}
	}
	return nil
}

// ModelListURL returns the URL the model list is fetched from, or "" when the
// harness configures neither Endpoint nor ModelURL.
//
// An explicit ModelURL wins. Otherwise the path is appended to Endpoint, which
// may be given with or without a trailing "/v1" — agent base URLs are commonly
// written both ways.
func (h *HarnessConfig) ModelListURL() string {
	if h.ModelURL != "" {
		return h.ModelURL
	}
	base := strings.TrimRight(h.Endpoint, "/")
	if base == "" {
		return ""
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/models"
	}
	return base + DefaultModelsPath
}

func (c *RootConfig) GetHarness(name string) (*HarnessConfig, error) {
	harness, ok := c.Harnesses[name]
	if !ok {
		return nil, fmt.Errorf("harness %q not found", name)
	}
	return &harness, nil
}

// ListHarnesses returns the configured harness names in sorted order.
func (c *RootConfig) ListHarnesses() []string {
	return sortedKeys(c.Harnesses)
}

func sortedKeys(harnesses map[string]HarnessConfig) []string {
	names := make([]string, 0, len(harnesses))
	for name := range harnesses {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// SortedEnvVars returns the harness env var names in sorted order, so that
// display output is stable across runs.
func (h *HarnessConfig) SortedEnvVars() []string {
	keys := make([]string, 0, len(h.EnvVars))
	for k := range h.EnvVars {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// ResolvedEnvVars returns EnvVars with placeholders substituted, which is what
// the agent actually receives. Entries whose placeholders cannot be resolved
// are omitted.
func (h *HarnessConfig) ResolvedEnvVars() map[string]string {
	out, _ := h.resolveEnvVars()
	return out
}

// UnresolvedEnvVars lists envVars entries dropped because a placeholder had no
// value, so the caller can say so rather than failing silently.
func (h *HarnessConfig) UnresolvedEnvVars() []string {
	_, dropped := h.resolveEnvVars()
	return dropped
}

func (h *HarnessConfig) resolveEnvVars() (map[string]string, []string) {
	vars := map[string]string{PlaceholderEndpoint: strings.TrimRight(h.Endpoint, "/")}
	out := make(map[string]string, len(h.EnvVars))
	var dropped []string
	for k, raw := range h.EnvVars {
		if v, ok := expandPlaceholders(raw, vars); ok {
			out[k] = v
		} else {
			dropped = append(dropped, k)
		}
	}
	slices.Sort(dropped)
	return out, dropped
}

func (h *HarnessConfig) BuildEnv() []string {
	resolved := h.ResolvedEnvVars()
	env := os.Environ()
	for _, k := range h.SortedEnvVars() {
		v, ok := resolved[k]
		if !ok {
			// Unresolvable placeholder: drop the variable rather than export a
			// literal "{{endpoint}}". Validation rejects this at load time.
			continue
		}
		prefix := k + "="
		found := false
		for i, e := range env {
			if strings.HasPrefix(e, prefix) {
				env[i] = prefix + v
				found = true
				break
			}
		}
		if !found {
			env = append(env, prefix+v)
		}
	}
	return env
}

// CombinedArgs returns the harness's configured args, then the model args with
// placeholders substituted from vars, then the user's args. It always
// allocates, so the config's own slices are never written through.
func (h *HarnessConfig) CombinedArgs(vars map[string]string, userArgs []string) []string {
	return slices.Concat(h.Args, h.expandModelArgs(vars), userArgs)
}

// expandModelArgs substitutes placeholders in ModelArgs. An argument naming a
// placeholder that has no value is dropped whole, so a half-substituted flag
// never reaches the agent. Writing flag and value as one token —
// "--config=model_context_window={{contextWindow}}" — keeps that drop clean.
func (h *HarnessConfig) expandModelArgs(vars map[string]string) []string {
	if len(vars) == 0 || len(h.ModelArgs) == 0 {
		return nil
	}
	out := make([]string, 0, len(h.ModelArgs))
	for _, arg := range h.ModelArgs {
		expanded, ok := expandPlaceholders(arg, vars)
		if !ok {
			continue
		}
		out = append(out, expanded)
	}
	return out
}

// expandPlaceholders replaces every {{name}} in arg. It reports false when any
// placeholder has no value, meaning the caller should drop the argument.
func expandPlaceholders(arg string, vars map[string]string) (string, bool) {
	complete := true
	out := placeholderPattern.ReplaceAllStringFunc(arg, func(token string) string {
		name := placeholderPattern.FindStringSubmatch(token)[1]
		if rest, found := strings.CutPrefix(name, envPlaceholderPrefix); found {
			if v := os.Getenv(rest); v != "" {
				return v
			}
			complete = false
			return token
		}
		value, ok := vars[name]
		if !ok || value == "" {
			complete = false
			return token
		}
		return value
	})
	return out, complete
}

// ApplyModel records the selected model in the harness environment, for
// harnesses that take their model through env vars.
func (h *HarnessConfig) ApplyModel(model string) {
	if model == "" || len(h.ModelEnvVars) == 0 {
		return
	}
	// A harness that declares no envVars unmarshals to a nil map.
	if h.EnvVars == nil {
		h.EnvVars = map[string]string{}
	}
	for _, name := range h.ModelEnvVars {
		h.EnvVars[name] = model
	}
}

// ResolvedModelAuthToken returns the bearer token to send when fetching the
// model list, with placeholders substituted, or "" when the harness configures
// none and the request should go out unauthenticated.
func (h *HarnessConfig) ResolvedModelAuthToken() string {
	if h.ModelAuthToken == "" {
		return ""
	}
	vars := map[string]string{PlaceholderEndpoint: strings.TrimRight(h.Endpoint, "/")}
	v, ok := expandPlaceholders(h.ModelAuthToken, vars)
	if !ok {
		return ""
	}
	return v
}

// ConfigPath returns the path of the config file to load.
func ConfigPath() string {
	if testConfigPath != "" {
		return testConfigPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return DefaultConfigFile
	}
	return filepath.Join(home, DefaultConfigDir, DefaultConfigFile)
}
