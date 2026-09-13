package harness

import (
	"os"
	"testing"
)

func TestLaunch_Success(t *testing.T) {
	result, err := Launch("echo", []string{"hello"}, []string{"PATH=" + os.Getenv("PATH")})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", result.ExitCode)
	}
}

func TestLaunch_NonZeroExit(t *testing.T) {
	result, err := Launch("sh", []string{"-c", "exit 3"}, []string{"PATH=" + os.Getenv("PATH")})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.ExitCode != 3 {
		t.Errorf("Expected exit code 3, got: %d", result.ExitCode)
	}
}

func TestLaunch_InvalidCommand(t *testing.T) {
	_, err := Launch("nonexistent-command-12345", []string{}, []string{})
	if err == nil {
		t.Fatal("Expected error for invalid command")
	}
}

func TestMaskValue(t *testing.T) {
	// Enough of the tail to tell two credentials apart, and nothing more.
	if got := MaskValue("sk-ant-api03-abcdefghijklmnopqrstuvwxyz123456"); got != "********3456" {
		t.Errorf("MaskValue() = %q, want ********3456", got)
	}
	// Short values reveal nothing: the length is itself a hint.
	if got := MaskValue("abc"); got != "********" {
		t.Errorf("MaskValue() = %q, want ********", got)
	}
	if got := MaskValue("12345678"); got != "********" {
		t.Errorf("MaskValue() = %q, want ******** for a short value", got)
	}
}

// Only credentials are hidden. Masking a base URL or a model id makes the
// launch banner unreadable without protecting anything.
func TestDisplayValue(t *testing.T) {
	cases := []struct{ name, value, want string }{
		{"ANTHROPIC_AUTH_TOKEN", "sk-secret-value-1234", "********1234"},
		{"OMLX_API_KEY", "sk-secret-value-1234", "********1234"},
		{"MY_PASSWORD", "hunter2hunter2", "********ter2"},
		{"AWS_SECRET_ACCESS_KEY", "abcdefghijklmnop", "********mnop"},
		{"ANTHROPIC_BASE_URL", "http://127.0.0.1:4000", "http://127.0.0.1:4000"},
		{"ANTHROPIC_DEFAULT_SONNET_MODEL", "qwen3-coder-30b", "qwen3-coder-30b"},
		{"API_TIMEOUT_MS", "3000000", "3000000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DisplayValue(tc.name, tc.value); got != tc.want {
				t.Errorf("DisplayValue(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}
