package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoadConfig_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	content := `{"harnesses":{"claude":{"executable":"claude","args":["--permission-mode","bypassPermissions"],"envVars":{"ANTHROPIC_BASE_URL":"http://127.0.0.1:4000"}}}}`
	os.WriteFile(configPath, []byte(content), 0644)
	testConfigPath = configPath
	defer func() { testConfigPath = "" }()
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(cfg.Harnesses) != 1 {
		t.Errorf("Expected 1 harness, got: %d", len(cfg.Harnesses))
	}
	h, err := cfg.GetHarness("claude")
	if err != nil {
		t.Fatalf("Expected to find claude harness, got: %v", err)
	}
	if h.Executable != "claude" {
		t.Errorf("Expected executable 'claude', got: %s", h.Executable)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testConfigPath = filepath.Join(tmpDir, "nonexistent.json")
	defer func() { testConfigPath = "" }()
	_, err := LoadConfig()
	if err == nil {
		t.Fatal("Expected error for missing config file")
	}
}

func TestValidateConfig_EmptyHarnesses(t *testing.T) {
	config := &RootConfig{Harnesses: map[string]HarnessConfig{}}
	if err := ValidateConfig(config); err == nil {
		t.Fatal("Expected error for empty harnesses")
	}
}

func TestValidateConfig_MissingExecutable(t *testing.T) {
	config := &RootConfig{Harnesses: map[string]HarnessConfig{"test": {Args: []string{"--help"}}}}
	if err := ValidateConfig(config); err == nil {
		t.Fatal("Expected error for missing executable")
	}
}

func TestGetHarness_NotFound(t *testing.T) {
	config := &RootConfig{Harnesses: map[string]HarnessConfig{}}
	if _, err := config.GetHarness("nonexistent"); err == nil {
		t.Fatal("Expected error for non-existent harness")
	}
}

func TestListHarnesses(t *testing.T) {
	config := &RootConfig{Harnesses: map[string]HarnessConfig{"claude": {Executable: "claude"}, "codex": {Executable: "codex"}}}
	if len(config.ListHarnesses()) != 2 {
		t.Errorf("Expected 2 harnesses, got: %d", len(config.ListHarnesses()))
	}
}

func TestBuildEnv(t *testing.T) {
	h := HarnessConfig{Executable: "claude", EnvVars: map[string]string{"API_KEY": "secret123"}}
	env := h.BuildEnv()
	found := false
	for _, e := range env {
		if e == "API_KEY=secret123" {
			found = true
		}
	}
	if !found {
		t.Error("Expected API_KEY in environment")
	}
}

func TestCombinedArgs(t *testing.T) {
	h := HarnessConfig{Executable: "claude", Args: []string{"--permission-mode", "bypassPermissions"}}
	allArgs := h.CombinedArgs(nil, []string{"--resume", "xxxx"})
	if len(allArgs) != 4 {
		t.Errorf("Expected 4 args, got: %d", len(allArgs))
	}
	if allArgs[2] != "--resume" || allArgs[3] != "xxxx" {
		t.Errorf("Expected user args at end, got: %v", allArgs)
	}
}

func TestCombinedArgs_SubstitutesModel(t *testing.T) {
	h := HarnessConfig{
		Executable: "codex",
		Args:       []string{"-c", "model_provider=omlx"},
		ModelArgs:  []string{"-m", ModelPlaceholder},
	}
	got := h.CombinedArgs(map[string]string{PlaceholderModel: "glm-4.6"}, []string{"--search"})
	want := []string{"-c", "model_provider=omlx", "-m", "glm-4.6", "--search"}
	if !slices.Equal(got, want) {
		t.Errorf("CombinedArgs = %v, want %v", got, want)
	}
}

// A placeholder must never reach the child process unreplaced.
func TestCombinedArgs_DropsModelArgsWhenNoModel(t *testing.T) {
	h := HarnessConfig{Executable: "pi", ModelArgs: []string{"--model", "omlx/" + ModelPlaceholder}}
	got := h.CombinedArgs(nil, []string{"chat"})
	want := []string{"chat"}
	if !slices.Equal(got, want) {
		t.Errorf("CombinedArgs = %v, want %v", got, want)
	}
}

func TestCombinedArgs_SubstitutesInsideArg(t *testing.T) {
	h := HarnessConfig{Executable: "pi", ModelArgs: []string{"--model", "omlx/" + ModelPlaceholder}}
	got := h.CombinedArgs(map[string]string{PlaceholderModel: "qwen3-coder-30b"}, nil)
	want := []string{"--model", "omlx/qwen3-coder-30b"}
	if !slices.Equal(got, want) {
		t.Errorf("CombinedArgs = %v, want %v", got, want)
	}
}

// CombinedArgs must not write through into the config's own slices.
func TestCombinedArgs_DoesNotMutateConfig(t *testing.T) {
	h := HarnessConfig{
		Executable: "codex",
		Args:       []string{"-c", "x=1"},
		ModelArgs:  []string{"-m", ModelPlaceholder},
	}
	h.CombinedArgs(map[string]string{PlaceholderModel: "model-a"}, []string{"extra"})
	h.CombinedArgs(map[string]string{PlaceholderModel: "model-b"}, []string{"other"})
	if !slices.Equal(h.Args, []string{"-c", "x=1"}) {
		t.Errorf("Args mutated: %v", h.Args)
	}
	if !slices.Equal(h.ModelArgs, []string{"-m", ModelPlaceholder}) {
		t.Errorf("ModelArgs mutated: %v", h.ModelArgs)
	}
}

func TestApplyModel(t *testing.T) {
	h := HarnessConfig{
		Executable:   "claude",
		EnvVars:      map[string]string{"ANTHROPIC_AUTH_TOKEN": "sk-test-token"},
		ModelEnvVars: []string{"ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL"},
	}
	h.ApplyModel("glm-4.6")
	for _, k := range h.ModelEnvVars {
		if h.EnvVars[k] != "glm-4.6" {
			t.Errorf("%s = %q, want glm-4.6", k, h.EnvVars[k])
		}
	}
	if h.EnvVars["ANTHROPIC_AUTH_TOKEN"] != "sk-test-token" {
		t.Error("ApplyModel clobbered an unrelated env var")
	}
}

// A harness declaring no envVars unmarshals to a nil map; assigning to it
// would panic.
func TestApplyModel_NilEnvVars(t *testing.T) {
	h := HarnessConfig{Executable: "claude", ModelEnvVars: []string{"MODEL"}}
	h.ApplyModel("m1")
	if h.EnvVars["MODEL"] != "m1" {
		t.Errorf("MODEL = %q, want m1", h.EnvVars["MODEL"])
	}
}

func TestApplyModel_NoModelOrNoVars(t *testing.T) {
	h := HarnessConfig{Executable: "claude", ModelEnvVars: []string{"MODEL"}}
	h.ApplyModel("")
	if len(h.EnvVars) != 0 {
		t.Errorf("empty model should set nothing, got %v", h.EnvVars)
	}

	h2 := HarnessConfig{Executable: "codex", EnvVars: map[string]string{"A": "b"}}
	h2.ApplyModel("m1")
	if len(h2.EnvVars) != 1 {
		t.Errorf("no modelEnvVars should set nothing, got %v", h2.EnvVars)
	}
}

func TestResolvedModelAuthToken_Literal(t *testing.T) {
	h := HarnessConfig{ModelAuthToken: "sk-literal"}
	if got := h.ResolvedModelAuthToken(); got != "sk-literal" {
		t.Errorf("= %q, want sk-literal", got)
	}
}

// The point of the field: keep the secret in the environment, not the file.
func TestResolvedModelAuthToken_FromEnvironment(t *testing.T) {
	t.Setenv("OMLX_API_KEY", "sk-from-shell")
	h := HarnessConfig{ModelAuthToken: "{{env:OMLX_API_KEY}}"}
	if got := h.ResolvedModelAuthToken(); got != "sk-from-shell" {
		t.Errorf("= %q, want sk-from-shell", got)
	}
}

// An unset source must yield no token, never the literal placeholder, which a
// server rejects with a confusing 401.
func TestResolvedModelAuthToken_UnresolvedYieldsNothing(t *testing.T) {
	t.Setenv("MISSING_KEY", "")
	h := HarnessConfig{ModelAuthToken: "{{env:MISSING_KEY}}"}
	if got := h.ResolvedModelAuthToken(); got != "" {
		t.Errorf("= %q, want empty", got)
	}
}

// No implicit lookup: a harness that names no token sends none, whatever
// envVars happens to hold or the shell happens to export.
func TestResolvedModelAuthToken_NoImplicitLookup(t *testing.T) {
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "sk-from-environment")
	h := HarnessConfig{EnvVars: map[string]string{"ANTHROPIC_AUTH_TOKEN": "sk-anthropic"}}
	if got := h.ResolvedModelAuthToken(); got != "" {
		t.Errorf("no modelAuthToken should mean no token, got %q", got)
	}
}

func TestValidateConfig_RejectsMistypedPlaceholderInModelAuthToken(t *testing.T) {
	cfg := &RootConfig{Harnesses: map[string]HarnessConfig{"h": {
		Executable:     "x",
		ModelAuthToken: "((OMLX_API_KEY))",
	}}}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected ((name)) to be rejected in modelAuthToken")
	}
	if !strings.Contains(err.Error(), "{{name}}") {
		t.Errorf("error should show the right syntax, got: %v", err)
	}
}

func TestValidateConfig_RejectsUnknownPlaceholderInModelAuthToken(t *testing.T) {
	cfg := &RootConfig{Harnesses: map[string]HarnessConfig{"h": {
		Executable:     "x",
		ModelAuthToken: "{{model}}",
	}}}
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("expected {{model}} to be rejected in modelAuthToken")
	}
}

func TestValidateConfig_ModelWiringNeedsModelURL(t *testing.T) {
	cases := map[string]HarnessConfig{
		"modelEnvVars": {Executable: "x", ModelEnvVars: []string{"M"}},
		"modelArgs":    {Executable: "x", ModelArgs: []string{"-m", ModelPlaceholder}},
	}
	for name, h := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := &RootConfig{Harnesses: map[string]HarnessConfig{"h": h}}
			if err := ValidateConfig(cfg); err == nil {
				t.Fatalf("expected an error for %s without modelURL", name)
			}
		})
	}
}

func TestCombinedArgs_ContextWindowPlaceholder(t *testing.T) {
	h := HarnessConfig{
		Executable: "codex",
		ModelArgs: []string{
			"-m", ModelPlaceholder,
			"--config=model_context_window={{contextWindow}}",
		},
	}
	got := h.CombinedArgs(map[string]string{
		PlaceholderModel:         "KAT-Coder",
		PlaceholderContextWindow: "262144",
	}, nil)
	want := []string{"-m", "KAT-Coder", "--config=model_context_window=262144"}
	if !slices.Equal(got, want) {
		t.Errorf("CombinedArgs = %v, want %v", got, want)
	}
}

// An endpoint that reports no context window must not produce a dangling
// "--config=model_context_window=" argument.
func TestCombinedArgs_DropsArgWithUnresolvedPlaceholder(t *testing.T) {
	h := HarnessConfig{
		Executable: "codex",
		ModelArgs: []string{
			"-m", ModelPlaceholder,
			"--config=model_context_window={{contextWindow}}",
		},
	}
	got := h.CombinedArgs(map[string]string{PlaceholderModel: "some-model"}, nil)
	want := []string{"-m", "some-model"}
	if !slices.Equal(got, want) {
		t.Errorf("CombinedArgs = %v, want %v", got, want)
	}
	for _, a := range got {
		if strings.Contains(a, "{{") {
			t.Errorf("unsubstituted placeholder reached the args: %q", a)
		}
	}
}

func TestValidateConfig_RejectsUnknownPlaceholder(t *testing.T) {
	cfg := &RootConfig{Harnesses: map[string]HarnessConfig{"h": {
		Executable: "x",
		ModelURL:   "http://localhost/v1/models",
		ModelArgs:  []string{"--limit={{maxTokens}}"},
	}}}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected an error for an unknown placeholder")
	}
	if !strings.Contains(err.Error(), "maxTokens") {
		t.Errorf("error should name the bad placeholder, got: %v", err)
	}
}

func TestValidateConfig_AcceptsKnownPlaceholders(t *testing.T) {
	cfg := &RootConfig{Harnesses: map[string]HarnessConfig{"h": {
		Executable: "x",
		ModelURL:   "http://localhost/v1/models",
		ModelArgs:  []string{"-m", ModelPlaceholder, "--config=model_context_window={{contextWindow}}"},
	}}}
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("expected known placeholders to validate, got: %v", err)
	}
}

func TestModelListURL(t *testing.T) {
	cases := []struct {
		name string
		in   HarnessConfig
		want string
	}{
		{"endpoint", HarnessConfig{Endpoint: "http://127.0.0.1:4000"}, "http://127.0.0.1:4000/v1/models"},
		{"endpoint trailing slash", HarnessConfig{Endpoint: "http://127.0.0.1:4000/"}, "http://127.0.0.1:4000/v1/models"},
		// Agent base URLs are commonly written with /v1 already on them.
		{"endpoint ending in /v1", HarnessConfig{Endpoint: "http://127.0.0.1:11434/v1"}, "http://127.0.0.1:11434/v1/models"},
		{"endpoint /v1/", HarnessConfig{Endpoint: "http://127.0.0.1:11434/v1/"}, "http://127.0.0.1:11434/v1/models"},
		{"path prefix", HarnessConfig{Endpoint: "https://gw.example.com/openai"}, "https://gw.example.com/openai/v1/models"},
		{"explicit modelURL", HarnessConfig{ModelURL: "http://h/api/tags"}, "http://h/api/tags"},
		{"modelURL wins over endpoint", HarnessConfig{Endpoint: "http://a", ModelURL: "http://b/api/tags"}, "http://b/api/tags"},
		{"neither", HarnessConfig{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.ModelListURL(); got != tc.want {
				t.Errorf("ModelListURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateConfig_ModelWiringAcceptsEndpoint(t *testing.T) {
	cfg := &RootConfig{Harnesses: map[string]HarnessConfig{"codex": {
		Executable: "codex",
		Endpoint:   "http://127.0.0.1:4000",
		ModelArgs:  []string{"-m", ModelPlaceholder},
	}}}
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("endpoint should satisfy the model-wiring requirement, got: %v", err)
	}
}

func TestValidateConfig_ModelWiringNeedsEndpointOrModelURL(t *testing.T) {
	cfg := &RootConfig{Harnesses: map[string]HarnessConfig{"codex": {
		Executable: "codex",
		ModelArgs:  []string{"-m", ModelPlaceholder},
	}}}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected an error when neither endpoint nor modelURL is set")
	}
	if !strings.Contains(err.Error(), "endpoint") {
		t.Errorf("error should mention endpoint, got: %v", err)
	}
}

func TestBuildEnv_ExpandsEndpointPlaceholder(t *testing.T) {
	h := HarnessConfig{
		Executable: "claude",
		Endpoint:   "http://127.0.0.1:4000",
		EnvVars: map[string]string{
			"ANTHROPIC_BASE_URL": "{{endpoint}}",
			"OPENAI_BASE_URL":    "{{endpoint}}/v1",
			"UNRELATED":          "literal",
		},
	}
	got := map[string]string{}
	for _, e := range h.BuildEnv() {
		if k, v, ok := strings.Cut(e, "="); ok {
			got[k] = v
		}
	}
	for k, want := range map[string]string{
		"ANTHROPIC_BASE_URL": "http://127.0.0.1:4000",
		"OPENAI_BASE_URL":    "http://127.0.0.1:4000/v1",
		"UNRELATED":          "literal",
	} {
		if got[k] != want {
			t.Errorf("%s = %q, want %q", k, got[k], want)
		}
	}
}

// A trailing slash on endpoint must not produce a doubled slash.
func TestBuildEnv_EndpointTrailingSlash(t *testing.T) {
	h := HarnessConfig{Endpoint: "http://h:4000/", EnvVars: map[string]string{"B": "{{endpoint}}/v1"}}
	for _, e := range h.BuildEnv() {
		if strings.HasPrefix(e, "B=") && e != "B=http://h:4000/v1" {
			t.Errorf("got %q, want B=http://h:4000/v1", e)
		}
	}
}

func TestValidateConfig_EnvVarsEndpointPlaceholderNeedsEndpoint(t *testing.T) {
	cfg := &RootConfig{Harnesses: map[string]HarnessConfig{"h": {
		Executable: "x",
		EnvVars:    map[string]string{"ANTHROPIC_BASE_URL": "{{endpoint}}"},
	}}}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected an error when {{endpoint}} is used with no endpoint set")
	}
	if !strings.Contains(err.Error(), "endpoint") {
		t.Errorf("error should explain the problem, got: %v", err)
	}
}

// {{model}} is not resolvable at envVars time; modelEnvVars covers that.
func TestValidateConfig_RejectsModelPlaceholderInEnvVars(t *testing.T) {
	cfg := &RootConfig{Harnesses: map[string]HarnessConfig{"h": {
		Executable: "x",
		Endpoint:   "http://h",
		EnvVars:    map[string]string{"SOME_MODEL": ModelPlaceholder},
	}}}
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("expected {{model}} to be rejected inside envVars")
	}
}

func TestValidateConfig_AcceptsEndpointPlaceholderInEnvVars(t *testing.T) {
	cfg := &RootConfig{Harnesses: map[string]HarnessConfig{"h": {
		Executable: "x",
		Endpoint:   "http://127.0.0.1:4000",
		EnvVars:    map[string]string{"ANTHROPIC_BASE_URL": "{{endpoint}}"},
	}}}
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("expected this to validate, got: %v", err)
	}
}

func TestBuildEnv_EnvPlaceholder(t *testing.T) {
	t.Setenv("OMLX_API_KEY", "sk-from-shell")
	h := HarnessConfig{
		Endpoint: "http://h:4000",
		EnvVars: map[string]string{
			"ANTHROPIC_AUTH_TOKEN": "{{env:OMLX_API_KEY}}",
			"MIXED":                "{{endpoint}}|{{env:OMLX_API_KEY}}",
		},
	}
	got := h.ResolvedEnvVars()
	if got["ANTHROPIC_AUTH_TOKEN"] != "sk-from-shell" {
		t.Errorf("= %q, want sk-from-shell", got["ANTHROPIC_AUTH_TOKEN"])
	}
	if got["MIXED"] != "http://h:4000|sk-from-shell" {
		t.Errorf("= %q, want the two substituted", got["MIXED"])
	}
}

// An unset source variable must drop the entry, never export a literal
// "{{env:...}}" as if it were a credential.
func TestBuildEnv_EnvPlaceholderUnsetIsDropped(t *testing.T) {
	t.Setenv("MISSING_KEY", "")
	h := HarnessConfig{EnvVars: map[string]string{"TOKEN": "{{env:MISSING_KEY}}"}}
	if v, ok := h.ResolvedEnvVars()["TOKEN"]; ok {
		t.Errorf("TOKEN should be dropped, got %q", v)
	}
	if got := h.UnresolvedEnvVars(); !slices.Equal(got, []string{"TOKEN"}) {
		t.Errorf("UnresolvedEnvVars() = %v, want [TOKEN]", got)
	}
}

// ((name)) is a plausible guess at the syntax; exporting it verbatim as a
// token produces a confusing 401 rather than a config error.
func TestValidateConfig_RejectsMistypedPlaceholder(t *testing.T) {
	cfg := &RootConfig{Harnesses: map[string]HarnessConfig{"claude": {
		Executable: "claude",
		Endpoint:   "http://h",
		EnvVars:    map[string]string{"ANTHROPIC_AUTH_TOKEN": "((modelAuthEnvVar))"},
	}}}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected ((name)) to be rejected")
	}
	if !strings.Contains(err.Error(), "{{name}}") {
		t.Errorf("error should show the right syntax, got: %v", err)
	}
}

func TestValidateConfig_AcceptsEnvPlaceholder(t *testing.T) {
	cfg := &RootConfig{Harnesses: map[string]HarnessConfig{"claude": {
		Executable: "claude",
		EnvVars:    map[string]string{"ANTHROPIC_AUTH_TOKEN": "{{env:OMLX_API_KEY}}"},
	}}}
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("expected {{env:NAME}} to validate, got: %v", err)
	}
}
