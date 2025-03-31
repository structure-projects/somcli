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
	"github.com/structure-projects/somcli/pkg/offline"
	"github.com/structure-projects/somcli/pkg/utils"
)

var (
	offlineDownloadConfig string
	offlineQuiet          bool
)

var offlineCmd = &cobra.Command{
	Use:   "offline",
	Short: "Offline resource management",
	Long:  `Manage offline resources download and cache`,
}

var offlineDownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download offline resources",
	Long:  `Download all required resources based on configuration file`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := executeOfflineDownload(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(offlineCmd)
	offlineCmd.AddCommand(offlineDownloadCmd)

	offlineDownloadCmd.Flags().StringVarP(&offlineDownloadConfig, "file", "f", "", "Download configuration file (required)")
	offlineDownloadCmd.Flags().BoolVarP(&offlineQuiet, "quiet", "q", false, "Quiet mode")

	offlineDownloadCmd.MarkFlagRequired("file")
}

func executeOfflineDownload() error {
	config, err := offline.LoadDownloadConfig(offlineDownloadConfig)
	utils.PrintInfo("config -> %s", config)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	results, err := offline.DownloadResources(config, offlineQuiet)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	printDownloadResults(results)
	return nil
}

func printDownloadResults(results []offline.DownloadResult) {
	fmt.Println("\nDownload results:")

	success := 0
	for _, res := range results {
		if res.Error == nil {
			success++
			fmt.Printf("  ✓ %s-%s: %s\n", res.Name, res.Version, res.LocalPath)
		} else {
			fmt.Printf("  ✗ %s-%s: %v\n", res.Name, res.Version, res.Error)
		}
	}

	fmt.Printf("\nSummary: %d/%d succeeded\n", success, len(results))
}
