package harness

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// LaunchResult contains information about the launched process.
type LaunchResult struct {
	ExitCode int
}

// Launch spawns a subprocess with the given command, arguments, and environment.
func Launch(command string, args []string, env []string) (*LaunchResult, error) {
	cmd := exec.Command(command, args...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	// Wait for the process to complete
	if err := cmd.Wait(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return &LaunchResult{ExitCode: exitError.ExitCode()}, nil
		}
		return nil, fmt.Errorf("process failed: %w", err)
	}

	return &LaunchResult{ExitCode: 0}, nil
}

// MaskValue hides sensitive parts of a value for display.
func MaskValue(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return value[:2] + "****" + value[len(value)-2:]
}
