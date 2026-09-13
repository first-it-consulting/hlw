package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/first-it-consulting/hlw/internal/config"
	"github.com/first-it-consulting/hlw/internal/harness"
	"github.com/first-it-consulting/hlw/internal/models"
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
				fmt.Printf("Warning: Failed to fetch models: %v\n", err)
				if h.ModelAuthToken == "" {
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
