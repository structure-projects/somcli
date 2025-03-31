/*
Copyright 2023 Structure Projects

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/structure-projects/somcli/pkg/images"
)

var (
	scope      string
	repo       string
	customFile string
	inputFile  string
	outputFile string
)

var imagesCmd = &cobra.Command{
	Use:   "docker-images",
	Short: "Manage Docker images lifecycle",
	Long:  `The docker-images command provides full lifecycle management for Docker images including pull, push, export and import operations.`,
}

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull images from registry",
	Run: func(cmd *cobra.Command, args []string) {
		config := images.Config{
			Scope:      scope,
			Repo:       repo,
			CustomFile: customFile,
			OutputFile: outputFile,
		}
		if err := images.Pull(config); err != nil {
			fmt.Printf("Error pulling images: %v\n", err)
			os.Exit(1)
		}
	},
}

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push images to registry",
	Run: func(cmd *cobra.Command, args []string) {
		config := images.Config{
			Scope:     scope,
			Repo:      repo,
			InputFile: inputFile,
		}
		if err := images.Push(config); err != nil {
			fmt.Printf("Error pushing images: %v\n", err)
			os.Exit(1)
		}
	},
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export images to file",
	Run: func(cmd *cobra.Command, args []string) {
		config := images.Config{
			Scope:      scope,
			Repo:       repo,
			CustomFile: customFile,
			OutputFile: outputFile,
		}
		if err := images.Export(config); err != nil {
			fmt.Printf("Error exporting images: %v\n", err)
			os.Exit(1)
		}
	},
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import images from file",
	Run: func(cmd *cobra.Command, args []string) {
		config := images.Config{
			Scope:     scope,
			Repo:      repo,
			InputFile: inputFile,
		}
		if err := images.Import(config); err != nil {
			fmt.Printf("Error importing images: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	// Pull command flags
	pullCmd.Flags().StringVarP(&scope, "scope", "s", "all", "Image scope (harbor|k8s|all)")
	pullCmd.Flags().StringVarP(&repo, "repo", "r", "", "Target registry repository")
	pullCmd.Flags().StringVarP(&customFile, "file", "f", "", "Custom image list file")
	pullCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file for pulled images list")

	// Push command flags
	pushCmd.Flags().StringVarP(&scope, "scope", "s", "all", "Image scope (harbor|k8s|all)")
	pushCmd.Flags().StringVarP(&repo, "repo", "r", "", "Target registry repository")
	pushCmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input file with images list")

	// Export command flags
	exportCmd.Flags().StringVarP(&scope, "scope", "s", "all", "Image scope (harbor|k8s|all)")
	exportCmd.Flags().StringVarP(&repo, "repo", "r", "", "Source registry repository")
	exportCmd.Flags().StringVarP(&customFile, "file", "f", "", "Custom image list file")
	exportCmd.Flags().StringVarP(&outputFile, "output", "o", "images.tar.gz", "Output archive file")

	// Import command flags
	importCmd.Flags().StringVarP(&scope, "scope", "s", "all", "Image scope (harbor|k8s|all)")
	importCmd.Flags().StringVarP(&repo, "repo", "r", "", "Target registry repository")
	importCmd.Flags().StringVarP(&inputFile, "input", "i", "images.tar.gz", "Input archive file")

	// Add subcommands
	imagesCmd.AddCommand(pullCmd)
	imagesCmd.AddCommand(pushCmd)
	imagesCmd.AddCommand(exportCmd)
	imagesCmd.AddCommand(importCmd)

	// Add to root command
	rootCmd.AddCommand(imagesCmd)
}
