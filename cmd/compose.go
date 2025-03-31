package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/structure-projects/somcli/pkg/compose"
)

func NewComposeCmd() *cobra.Command {
	var (
		silent      bool
		installPath string
		proxy       string
		envFile     string
	)

	rootCmd := &cobra.Command{
		Use:     "docker-compose",
		Aliases: []string{"compose", "dc"},
		Short:   "Enhanced Docker Compose management",
		Long: `Enhanced Docker Compose wrapper with additional features:

* Auto-install if not present
* Smart command defaults (e.g. 'up' defaults to -d)
* Proxy support for installation
* Environment file support
* Native signal handling

Examples:
  somcli docker-compose up          # Auto -d
  somcli docker-compose ps          # Auto -a
  somcli docker-compose logs -f     # Passthrough with signals
  somcli docker-compose --env-file .env.prod up`,
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			if envFile != "" {
				if _, err := os.Stat(envFile); err == nil {
					os.Setenv("COMPOSE_FILE", envFile)
				}
			}

			installer := compose.NewInstaller(silent, viper.GetViper())
			if installPath != "" {
				installer.SetInstallPath(installPath)
			}

			filteredArgs := filterArgs(args, envFile != "")
			if err := installer.Passthrough(filteredArgs); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	rootCmd.Flags().StringVarP(&envFile, "env-file", "e", "", "Specify an alternate environment file")
	rootCmd.PersistentFlags().BoolVarP(&silent, "yes", "y", false, "Automatic yes to prompts")
	rootCmd.PersistentFlags().StringVar(&installPath, "path", "", "Custom installation path")
	// rootCmd.PersistentFlags().StringVarP(&proxy, "proxy", "p", "", "Proxy server for installation")
	addDockerComposeSubcommands(rootCmd, &silent, &installPath, &proxy)

	return rootCmd
}

func addDockerComposeSubcommands(rootCmd *cobra.Command, silent *bool, installPath *string, proxy *string) {
	installCmd := &cobra.Command{
		Use:   "install [version]",
		Short: "Install Docker Compose",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			version := "2.24.0"
			if len(args) > 0 {
				version = args[0]
			}

			installer := compose.NewInstaller(*silent, viper.GetViper())
			if *installPath != "" {
				installer.SetInstallPath(*installPath)
			}
			return installer.Install(version)
		},
	}

	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Docker Compose",
		RunE: func(cmd *cobra.Command, args []string) error {
			installer := compose.NewInstaller(*silent, viper.GetViper())
			if *installPath != "" {
				installer.SetInstallPath(*installPath)
			}
			return installer.Uninstall()
		},
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show Docker Compose version",
		RunE: func(cmd *cobra.Command, args []string) error {
			installer := compose.NewInstaller(*silent, viper.GetViper())
			if *installPath != "" {
				installer.SetInstallPath(*installPath)
			}

			version, err := installer.Version()
			if err != nil {
				return err
			}

			fmt.Printf("Docker Compose version: %s\n", version)
			return nil
		},
	}

	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(versionCmd)
}

func filterArgs(args []string, hasEnvFile bool) []string {
	var filtered []string
	skipNext := false

	for i, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}

		if strings.HasPrefix(arg, "-") {
			switch arg {
			case "-y", "--yes", "-p", "--proxy", "--path", "-e", "--env-file":
				skipNext = true
				continue
			}
		}

		filtered = append(filtered, args[i])
	}

	return filtered
}
