package harness

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
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

// secretName matches environment variable names that hold credentials. Only
// those are masked: hiding a base URL or a model id makes the launch banner
// harder to read without protecting anything.
var secretName = regexp.MustCompile(`(?i)(TOKEN|KEY|SECRET|PASSWORD|PASSWD|CREDENTIAL)`)

// IsSecret reports whether a variable of this name should have its value
// hidden when printed.
func IsSecret(name string) bool {
	return secretName.MatchString(name)
}

// MaskValue hides a secret value for display, keeping just enough to tell two
// credentials apart.
func MaskValue(value string) string {
	if len(value) <= 8 {
		return "********"
	}
	return "********" + value[len(value)-4:]
}

// DisplayValue returns the value to print for a variable: masked when the name
// says it is a credential, shown in full otherwise.
func DisplayValue(name, value string) string {
	if IsSecret(name) {
		return MaskValue(value)
	}
	return value
}
