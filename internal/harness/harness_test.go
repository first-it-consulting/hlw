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

func TestMaskValue_LongValue(t *testing.T) {
	value := "sk-ant-api03-abcdefghijklmnopqrstuvwxyz123456"
	masked := MaskValue(value)

	if masked == value {
		t.Error("Expected value to be masked")
	}

	// Should show first 2 and last 2 chars with **** in between
	expected := "sk****56"
	if masked != expected {
		t.Errorf("Expected '%s', got: %s", expected, masked)
	}
}

func TestMaskValue_ShortValue(t *testing.T) {
	value := "abc"
	masked := MaskValue(value)

	if masked != "****" {
		t.Errorf("Expected '****' for short value, got: %s", masked)
	}
}
