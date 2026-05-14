package commands

import (
	"os"

	"github.com/spf13/cobra"
)

var app = &App{
	Writer: os.Stdout,
	Output: "table",
}

var rootCmd = &cobra.Command{
	Use:   "brainsentry",
	Short: "Brain Sentry memory management CLI",
	Long:  "CLI tool for managing Brain Sentry second-brain memories.",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&app.TenantID, "tenant", "a9f814d2-4dae-41f3-851b-8aa3d4706561", "Tenant ID")
	rootCmd.PersistentFlags().StringVarP(&app.Output, "output", "o", "table", "Output format: table, json, plain")

	rootCmd.AddCommand(
		newAddCmd(app),
		newSearchCmd(app),
		newListCmd(app),
		newEditCmd(app),
		newCorrectCmd(app),
		newReviewCmd(app),
		newImportCmd(app),
		newDoctorCmd(app),
		newModelsCmd(app),
		newEvalCmd(app),
		newInitCmd(app),
	)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
