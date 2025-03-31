package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/structure-projects/somcli/pkg/cluster"
	"github.com/structure-projects/somcli/pkg/resources"
	"github.com/structure-projects/somcli/pkg/utils"
)

var (
	allNamespaces bool
	namespace     string
	outputFormat  string
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get resources",
	Long:  `Get resources from Kubernetes, Docker Swarm or Docker clusters.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		resourceType := args[0]
		clusterType := cluster.DetectClusterType()
		if clusterType == cluster.TypeNone {
			utils.PrintError("No supported cluster detected")
			os.Exit(1)
		}
		result, err := resources.GetResources(clusterType, resourceType, namespace, allNamespaces, outputFormat)
		if err != nil {
			utils.PrintError("Failed to get resources: %v", err)
			os.Exit(1)
		}

		fmt.Println(result)
	},
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply configuration",
	Long:  `Apply configuration to Kubernetes, Docker Swarm or Docker clusters.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file := args[0]
		clusterType := cluster.DetectClusterType()
		if clusterType == cluster.TypeNone {
			utils.PrintError("No supported cluster detected")
			os.Exit(1)
		}

		if err := resources.ApplyResources(clusterType, file); err != nil {
			utils.PrintError("Failed to apply resources: %v", err)
			os.Exit(1)
		}

		utils.PrintSuccess("Resources applied successfully")
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete resources",
	Long:  `Delete resources from Kubernetes, Docker Swarm or Docker clusters.`,
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		resourceType := args[0]
		resourceName := args[1]
		clusterType := cluster.DetectClusterType()
		if clusterType == cluster.TypeNone {
			utils.PrintError("No supported cluster detected")
			os.Exit(1)
		}

		if err := resources.DeleteResource(clusterType, resourceType, resourceName, namespace); err != nil {
			utils.PrintError("Failed to delete resource: %v", err)
			os.Exit(1)
		}

		utils.PrintSuccess("Resource deleted successfully")
	},
}

// 在init函数前添加describeCmd
var describeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Show details of a specific resource",
	Long:  `Show detailed information about a specific resource in Kubernetes, Docker Swarm or Docker clusters.`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		resourceType := args[0]
		resourceName := args[1]
		clusterType := cluster.DetectClusterType()
		if clusterType == cluster.TypeNone {
			utils.PrintError("No supported cluster detected")
			os.Exit(1)
		}

		result, err := resources.DescribeResource(clusterType, resourceType, resourceName, namespace)
		if err != nil {
			utils.PrintError("Failed to describe resource: %v", err)
			os.Exit(1)
		}

		fmt.Println(result)
	},
}

func init() {
	// get 命令标志
	getCmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "All namespaces")
	getCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace")
	getCmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format")
	describeCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace")

	// 添加到根命令
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(describeCmd)
}
