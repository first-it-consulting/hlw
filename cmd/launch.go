package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/first-it-consulting/hlw/internal/config"
	"github.com/first-it-consulting/hlw/internal/harness"
	"github.com/first-it-consulting/hlw/internal/models"
	"github.com/first-it-consulting/hlw/internal/policy"
	"github.com/spf13/cobra"
)

// launchCmd represents the launch command.
var launchCmd = &cobra.Command{
	Use:   "launch <harness> [args...]",
	Short: "Launch an AI coding agent",
	Long: `Launch an AI coding agent using the specified harness configuration.

The tool will:
1. Load the harness configuration
2. Fetch available models from the endpoint (if modelURL is configured)
3. Display the model list for selection
4. Launch the agent with configured args, user-provided args, and environment variables

Example:
  hlw launch claude --resume xxxx
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		harnessName := args[0]
		userArgs := args[1:]

		// Load config
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Get harness configuration
		h, err := cfg.GetHarness(harnessName)
		if err != nil {
			return err
		}

		fmt.Printf("Using harness: %s\n", harnessName)
		fmt.Printf("Executable: %s\n\n", h.Executable)

		// Warn before the model picker, so nobody picks a model that a managed
		// policy is going to throw away.
		warnManagedPolicy(h)

		// Fetch model list if modelURL is configured
		modelVars := map[string]string{}
		if modelListURL := h.ModelListURL(); modelListURL != "" {
			fmt.Println("Fetching available models...")
			headers := map[string]string{}
			if token := h.ResolvedModelAuthToken(); token != "" {
				headers["Authorization"] = "Bearer " + token
			} else if h.ModelAuthToken != "" {
				fmt.Printf("Warning: modelAuthToken (%s) could not be resolved;\n", h.ModelAuthToken)
				fmt.Println("fetching the model list without authentication.")
			}
			modelList, err := models.FetchModels(modelListURL, headers)
			if err != nil {
				warnFetchFailed(h, modelListURL, err)
				if h.ModelAuthToken == "" && !models.Unreachable(err) {
					fmt.Println("Hint: no modelAuthToken is configured, so the request was")
					fmt.Println("sent unauthenticated. Set one (e.g. \"{{env:MY_API_KEY}}\") if")
					fmt.Println("the endpoint needs a token.")
				}
				fmt.Println("Continuing without model selection...")
			} else {
				model, rest, fromArg := takeModelArg(modelList.Models, userArgs)
				userArgs = rest
				if fromArg {
					fmt.Printf("Model from argument: %s\n\n", model.ID)
				}
				if !fromArg {
					model, err = models.SelectModel(modelList.Models)
					if errors.Is(err, models.ErrSelectionCanceled) {
						fmt.Println("Canceled.")
						os.Exit(130)
					}
					if err != nil {
						return fmt.Errorf("failed to select model: %w", err)
					}
					fmt.Printf("Selected model: %s\n\n", model.ID)
				}

				// Harnesses that take their model through the environment.
				h.ApplyModel(model.ID)

				// Values available to modelArgs placeholders.
				modelVars[config.PlaceholderModel] = model.ID
				if cw := model.ContextWindow(); cw > 0 {
					modelVars[config.PlaceholderContextWindow] = strconv.Itoa(cw)
				}
			}
		}

		// Build environment from harness config
		if dropped := h.UnresolvedEnvVars(); len(dropped) > 0 {
			for _, k := range dropped {
				fmt.Printf("Warning: envVars %q has an unresolved placeholder (%s); not set\n", k, h.EnvVars[k])
			}
		}
		env := h.BuildEnv()

		// Combine harness args, the model args, and user-provided args
		allArgs := h.CombinedArgs(modelVars, userArgs)

		// Claude Code lets the env block of its own settings files override
		// the process environment. A --settings file outranks those.
		cleanup := func() {}
		if h.EnvViaSettings {
			path, done, err := writeSettingsFile(h)
			if err != nil {
				return err
			}
			cleanup = done
			allArgs = append([]string{"--settings", path}, allArgs...)
		}

		// Print launch info
		fmt.Printf("Launching %s %v\n", h.Executable, allArgs)
		if len(h.EnvVars) > 0 {
			fmt.Println("Environment variables:")
			// Show what the agent actually receives, not the raw config value.
			resolved := h.ResolvedEnvVars()
			for _, k := range h.SortedEnvVars() {
				fmt.Printf("  %s=%s\n", k, harness.DisplayValue(k, resolved[k]))
			}
			fmt.Println()
		}

		// Launch the harness
		fmt.Println("Launching...")
		result, err := harness.Launch(h.Executable, allArgs, env)
		// The agent has exited, so its settings file is no longer read.
		cleanup()
		if err != nil {
			return fmt.Errorf("launch failed: %w", err)
		}

		if result.ExitCode != 0 {
			fmt.Printf("\nProcess exited with code %d\n", result.ExitCode)
			os.Exit(result.ExitCode)
		}

		return nil
	},
}

// completeHarnesses offers the configured harness names, annotated with their
// descriptions, for the first argument of "hlw launch".
func completeHarnesses(toComplete string) ([]string, cobra.ShellCompDirective) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var out []string
	for _, name := range cfg.ListHarnesses() {
		if !strings.HasPrefix(name, toComplete) {
			continue
		}
		h, err := cfg.GetHarness(name)
		if err != nil || h.Description == "" {
			out = append(out, name)
			continue
		}
		// Shells that support it show the text after the tab as a hint.
		out = append(out, name+"\t"+h.Description)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// takeModelArg consumes the first argument after the harness name when it
// names one of the endpoint's models, so "hlw launch claude <model>" skips the
// picker. Anything else — an agent flag, or a subcommand like "opencode run" —
// is left in place and passed through untouched.
func takeModelArg(available []models.Model, userArgs []string) (models.Model, []string, bool) {
	if len(userArgs) == 0 || strings.HasPrefix(userArgs[0], "-") {
		return models.Model{}, userArgs, false
	}
	m, ok := models.Find(available, userArgs[0])
	if !ok {
		return models.Model{}, userArgs, false
	}
	return m, userArgs[1:], true
}

// completionFetchTimeout keeps a slow or unreachable endpoint from stalling the
// shell prompt: completion gives up quickly and offers nothing.
const completionFetchTimeout = 2 * time.Second

// completeModels offers the model ids the harness's endpoint serves.
func completeModels(harnessName, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	h, err := cfg.GetHarness(harnessName)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	url := h.ModelListURL()
	if url == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	headers := map[string]string{}
	if token := h.ResolvedModelAuthToken(); token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	list, err := models.FetchModelsTimeout(url, headers, completionFetchTimeout)
	if err != nil {
		// Nothing to offer, but do not fall back to filenames: a filename is
		// never a model.
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, m := range list.Models {
		if strings.HasPrefix(m.ID, toComplete) {
			out = append(out, m.ID)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	// Stop parsing flags once the harness name is seen, so everything after it
	// is passed through to the agent rather than being claimed by hlw. Without
	// this, "hlw launch claude --resume x" fails with "unknown flag: --resume".
	launchCmd.Flags().SetInterspersed(false)

	launchCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return completeHarnesses(toComplete)
		}
		if len(args) == 1 {
			return completeModels(args[0], toComplete)
		}
		// Further arguments belong to the agent, so hlw has nothing useful to
		// offer and should not suggest files either.
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	rootCmd.AddCommand(launchCmd)
}

// warnManagedPolicy explains, for Claude Code harnesses, which parts of the
// launch a managed (enterprise) settings file will override. hlw cannot win
// against those settings, so it says so up front instead of letting the agent
// quietly connect somewhere else.
func warnManagedPolicy(h *config.HarnessConfig) {
	if !policy.IsClaude(h.Executable) {
		return
	}
	l := policy.Launch{
		Endpoint:     strings.TrimRight(h.Endpoint, "/"),
		Env:          h.ResolvedEnvVars(),
		ModelEnvVars: h.ModelEnvVars,
	}
	printPolicyConflicts(os.Stdout, policy.Conflicts(policy.FindClaude(), l))
	// User settings can be beaten, so only mention them when the harness
	// does not already do that.
	if !h.EnvViaSettings {
		printUserSettingsConflicts(os.Stdout, policy.UserSettingsConflicts(l))
	}
}

func printUserSettingsConflicts(w io.Writer, conflicts map[string][]string) {
	if len(conflicts) == 0 {
		return
	}
	fmt.Fprintln(w, "Warning: your Claude Code settings override this launch.")
	printConflictList(w, conflicts)
	fmt.Fprintln(w, "  Settings files outrank the environment hlw passes. Set")
	fmt.Fprintln(w, "  \"envViaSettings\": true on this harness to hand the values over as")
	fmt.Fprintln(w, "  --settings, which wins over them (or fix the settings file if hlw")
	fmt.Fprintln(w, "  could not read it).")
	fmt.Fprintln(w)
}

func printConflictList(w io.Writer, conflicts map[string][]string) {
	paths := make([]string, 0, len(conflicts))
	for p := range conflicts {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range paths {
		fmt.Fprintf(w, "  %s\n", p)
		for _, m := range conflicts[p] {
			fmt.Fprintf(w, "    - %s\n", m)
		}
	}
}

// writeSettingsFile writes the harness env as a Claude Code settings file and
// returns its path and a function that removes it. A file, not inline JSON,
// keeps tokens out of the process list.
func writeSettingsFile(h *config.HarnessConfig) (string, func(), error) {
	data, err := h.SettingsJSON()
	if err != nil {
		return "", nil, fmt.Errorf("failed to build settings: %w", err)
	}
	f, err := os.CreateTemp("", "hlw-settings-*.json") // created 0600
	if err != nil {
		return "", nil, fmt.Errorf("failed to create settings file: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, fmt.Errorf("failed to write settings file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", nil, fmt.Errorf("failed to write settings file: %w", err)
	}
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}

func printPolicyConflicts(w io.Writer, conflicts map[string][]string) {
	if len(conflicts) == 0 {
		return
	}
	fmt.Fprintln(w, "Warning: a Claude Code managed policy overrides this launch.")
	printConflictList(w, conflicts)
	fmt.Fprintln(w, "  Managed settings take precedence over everything hlw passes, including")
	fmt.Fprintln(w, "  --settings; hlw cannot override them. Ask whoever manages the policy to")
	fmt.Fprintln(w, "  allow your endpoint, or use a harness it does not cover (codex, opencode, pi).")
	fmt.Fprintln(w)
}

// warnFetchFailed tells "nothing is listening" apart from "the endpoint
// answered with an error", because the first means the agent will fail too.
func warnFetchFailed(h *config.HarnessConfig, url string, err error) {
	if models.Unreachable(err) {
		fmt.Printf("Warning: endpoint not reachable: %s\n", url)
		fmt.Printf("  %v\n", err)
		fmt.Println("  Is the local server or gateway running, and is the endpoint in the config right?")
		if h.Endpoint != "" {
			fmt.Printf("  The agent is pointed at %s too, so it will most likely fail to connect.\n", h.Endpoint)
		}
		return
	}
	fmt.Printf("Warning: Failed to fetch models: %v\n", err)
}
