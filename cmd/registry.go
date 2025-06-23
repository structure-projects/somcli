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
	"strings"

	"github.com/structure-projects/somcli/pkg/registry"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RegistryCmd 是registry命令的根命令
var RegistryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Manage container image registries",
	Long:  "Commands for managing container image registries including Harbor installation and image synchronization",
}

var (
	harborVersion string
	harborHost    string
	caPath        string
)

var registryInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Harbor registry",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !strings.Contains(harborHost, ".") && harborHost != "localhost" {
			return fmt.Errorf("invalid hostname format, must be a domain name or localhost")
		}
		if !strings.HasPrefix(harborVersion, "v") {
			return fmt.Errorf("harbor version must start with 'v'")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		manager := registry.NewHarborManager(
			harborVersion,
			harborHost,
			caPath,
			viper.GetViper(),
		)

		if err := manager.Install(); err != nil {
			fmt.Printf("Error installing Harbor: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Harbor installed successfully at %s\n", harborHost)
	},
}

var unInstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall Harbor registry",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !strings.Contains(harborHost, ".") && harborHost != "localhost" {
			return fmt.Errorf("invalid hostname format, must be a domain name or localhost")
		}
		if !strings.HasPrefix(harborVersion, "v") {
			return fmt.Errorf("harbor version must start with 'v'")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		manager := registry.NewHarborManager(
			harborVersion,
			harborHost,
			caPath,
			viper.GetViper(),
		)

		if err := manager.Uninstall(); err != nil {
			fmt.Printf("Error UnInstall Harbor: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Harbor UnInstalled successfully at %s\n", harborHost)
	},
}

var (
	sourceReg   string
	targetReg   string
	username    string
	password    string
	concurrency int
	imageList   string
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync images between registries",
	PreRunE: func(cmd *cobra.Command, args []string) error {

		if !strings.HasPrefix(targetReg, "http://") && !strings.HasPrefix(targetReg, "https://") {
			return fmt.Errorf("target registry must start with http:// or https://")
		}
		if concurrency < 1 || concurrency > 10 {
			return fmt.Errorf("concurrency must be between 1 and 10")
		}
		if _, err := os.Stat(imageList); os.IsNotExist(err) {
			return fmt.Errorf("image list file does not exist")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if password == "" {
			password = os.Getenv("REGISTRY_PASSWORD")
			if password == "" {
				fmt.Println("Error: password must be provided via -p flag or REGISTRY_PASSWORD environment variable")
				os.Exit(1)
			}
		}

		syncer := registry.NewRegistrySyncer(
			sourceReg,
			targetReg,
			username,
			password,
			concurrency,
		)

		images, err := readImageList(imageList)
		if err != nil {
			fmt.Printf("Error reading image list: %v\n", err)
			os.Exit(1)
		}

		if len(images) == 0 {
			fmt.Println("No images found in the image list file")
			os.Exit(1)
		}

		if err := syncer.SyncAll(images); err != nil {
			fmt.Printf("Error syncing images: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Image sync completed successfully")
	},
}

func readImageList(file string) ([]string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read image list file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var images []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			images = append(images, line)
		}
	}
	return images, nil
}

func init() {
	// sync命令参数
	syncCmd.Flags().StringVarP(&sourceReg, "source", "s", "", "Source registry URL (e.g. registry-1.docker.io) (required)")
	syncCmd.Flags().StringVarP(&targetReg, "target", "t", "", "Target registry URL (e.g. https://harbor.example.com) (required)")
	syncCmd.Flags().StringVarP(&username, "username", "u", "", "Registry username")
	syncCmd.Flags().StringVarP(&password, "password", "p", "", "Registry password (or use REGISTRY_PASSWORD env)")
	syncCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 3, "Number of concurrent sync operations (1-10)")
	syncCmd.Flags().StringVarP(&imageList, "image-list", "f", "", "Path to file containing list of images to sync (one per line) (required)")

	syncCmd.MarkFlagRequired("source")
	syncCmd.MarkFlagRequired("target")
	syncCmd.MarkFlagRequired("image-list")

	// install命令参数 - 确保短标志不重复
	registryInstallCmd.Flags().StringVarP(&harborVersion, "version", "v", "v2.5.0", "Harbor version to install (e.g. v2.5.0)")
	registryInstallCmd.Flags().StringVarP(&harborHost, "hostname", "H", "", "Harbor hostname (e.g. harbor.example.com) (required)") // 将 'h' 改为 'H'
	registryInstallCmd.Flags().StringVar(&caPath, "ca-path", "", "Path to CA certificate files directory")
	registryInstallCmd.MarkFlagRequired("hostname")

	RegistryCmd.AddCommand(syncCmd)
	RegistryCmd.AddCommand(registryInstallCmd)
	RegistryCmd.AddCommand(unInstallCmd)
	rootCmd.AddCommand(RegistryCmd)
}
