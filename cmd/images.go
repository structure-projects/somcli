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

// 这些标志按子命令拆开，而不是 pull/push/export/import 共用同一个包级变量：
// pflag 的 StringVar 在注册时就把默认值写进变量，而 pull/export 对 -o 的默认值不同
// （"" vs "images.tar.gz"）、push/import 对 -i 同理。共用会让后注册的默认值泄漏到
// 别的子命令 —— 于是 `images pull` 不带 -o 也会凭空写一个 images.tar.gz，
// `images push` 不带 -i 会去读一个不存在的 images.tar.gz（G9）。
var (
	scope            string
	repo             string
	customFile       string
	pullOutputFile   string
	exportOutputFile string
	pushInputFile    string
	importInputFile  string
)

var imagesCmd = &cobra.Command{
	Use:   "images",
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
			OutputFile: pullOutputFile,
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
			InputFile: pushInputFile,
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
			OutputFile: exportOutputFile,
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
			InputFile: importInputFile,
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
	pullCmd.Flags().StringVarP(&pullOutputFile, "output", "o", "", "Output file for pulled images list")

	// Push command flags
	pushCmd.Flags().StringVarP(&scope, "scope", "s", "all", "Image scope (harbor|k8s|all)")
	pushCmd.Flags().StringVarP(&repo, "repo", "r", "", "Target registry repository")
	pushCmd.Flags().StringVarP(&pushInputFile, "input", "i", "", "Input file with images list")

	// Export command flags
	exportCmd.Flags().StringVarP(&scope, "scope", "s", "all", "Image scope (harbor|k8s|all)")
	exportCmd.Flags().StringVarP(&repo, "repo", "r", "", "Source registry repository")
	exportCmd.Flags().StringVarP(&customFile, "file", "f", "", "Custom image list file")
	exportCmd.Flags().StringVarP(&exportOutputFile, "output", "o", "images.tar.gz", "Output archive file")

	// Import command flags
	importCmd.Flags().StringVarP(&scope, "scope", "s", "all", "Image scope (harbor|k8s|all)")
	importCmd.Flags().StringVarP(&repo, "repo", "r", "", "Target registry repository")
	importCmd.Flags().StringVarP(&importInputFile, "input", "i", "images.tar.gz", "Input archive file")

	// Add subcommands
	imagesCmd.AddCommand(pullCmd)
	imagesCmd.AddCommand(pushCmd)
	imagesCmd.AddCommand(exportCmd)
	imagesCmd.AddCommand(importCmd)

	// Add to root command
	rootCmd.AddCommand(imagesCmd)
}
