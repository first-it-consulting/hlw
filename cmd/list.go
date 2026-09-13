package cmd

import (
	"fmt"

	"github.com/first-it-consulting/hlw/internal/config"
	"github.com/spf13/cobra"
)

// listCmd represents the list command.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured harnesses",
	Long:  `Display a list of all configured harnesses.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		harnesses := cfg.ListHarnesses()
		if len(harnesses) == 0 {
			fmt.Println("No harnesses configured.")
			return nil
		}

		fmt.Printf("Configured harnesses (%d):\n\n", len(harnesses))
		for _, name := range harnesses {
			h, err := cfg.GetHarness(name)
			if err != nil {
				return err
			}

			fmt.Printf("  %-20s %s\n", name, h.Executable)
			if len(h.Args) > 0 {
				fmt.Printf("    Args:      %v\n", h.Args)
			}
			if len(h.EnvVars) > 0 {
				fmt.Printf("    EnvVars:   %d\n", len(h.EnvVars))
			}
			if url := h.ModelListURL(); url != "" {
				fmt.Printf("    Models:    %s\n", url)
			}
			if len(h.ModelArgs) > 0 {
				fmt.Printf("    ModelArgs: %v\n", h.ModelArgs)
			}
			if len(h.ModelEnvVars) > 0 {
				fmt.Printf("    ModelEnv:  %v\n", h.ModelEnvVars)
			}
			if h.Description != "" {
				fmt.Printf("    Description: %s\n", h.Description)
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
