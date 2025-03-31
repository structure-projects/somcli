package resources

import (
	"fmt"

	"github.com/structure-projects/somcli/pkg/cluster"
	"github.com/structure-projects/somcli/pkg/utils"
)

// GetResources 获取资源列表
func GetResources(clusterType cluster.ClusterType, resourceType, namespace string, allNamespaces bool, outputFormat string) (string, error) {
	normalizedType, err := DetectResourceType(resourceType)
	if err != nil {
		return "", fmt.Errorf("invalid resource type: %w", err)
	}

	switch clusterType {
	case cluster.TypeK8s:
		args := []string{"get", normalizedType}
		if allNamespaces {
			args = append(args, "-A")
		} else if namespace != "" {
			args = append(args, "-n", namespace)
		}
		if outputFormat != "" {
			args = append(args, "-o", outputFormat)
		}
		return utils.RunCommandWithOutput("kubectl", args...)

	case cluster.TypeSwarm:
		switch normalizedType {
		case "services":
			return utils.RunCommandWithOutput("docker", "service", "ls")
		case "nodes":
			return utils.RunCommandWithOutput("docker", "node", "ls")
		case "stacks":
			return utils.RunCommandWithOutput("docker", "stack", "ls")
		default:
			return "", fmt.Errorf("unsupported Swarm resource type: %s", resourceType)
		}

	case cluster.TypeDocker:
		switch normalizedType {
		case "containers":
			return utils.RunCommandWithOutput("docker", "ps", "-a")
		case "images":
			return utils.RunCommandWithOutput("docker", "images")
		default:
			return "", fmt.Errorf("unsupported Docker resource type: %s", resourceType)
		}

	default:
		return "", fmt.Errorf("unsupported cluster type: %s", clusterType)
	}
}

// ApplyResources 应用资源配置
func ApplyResources(clusterType cluster.ClusterType, file string) error {
	switch clusterType {
	case cluster.TypeK8s:
		return utils.RunCommand("kubectl", "apply", "-f", file)

	case cluster.TypeSwarm:
		stackName, composeFile, err := convertToStackFormat(file)
		if err != nil {
			return fmt.Errorf("failed to prepare Swarm stack: %w", err)
		}
		return utils.RunCommand("docker", "stack", "deploy", "-c", composeFile, stackName)

	case cluster.TypeDocker:
		return utils.RunCommand("docker-compose", "-f", file, "up", "-d")

	default:
		return fmt.Errorf("unsupported cluster type: %s", clusterType)
	}
}

// DeleteResource 删除资源
func DeleteResource(clusterType cluster.ClusterType, resourceType, resourceName, namespace string) error {
	normalizedType, err := DetectResourceType(resourceType)
	if err != nil {
		return fmt.Errorf("invalid resource type: %w", err)
	}

	switch clusterType {
	case cluster.TypeK8s:
		args := []string{"delete", normalizedType, resourceName}
		if namespace != "" {
			args = append(args, "-n", namespace)
		}
		return utils.RunCommand("kubectl", args...)

	case cluster.TypeSwarm:
		switch normalizedType {
		case "services":
			return utils.RunCommand("docker", "service", "rm", resourceName)
		case "stacks":
			return utils.RunCommand("docker", "stack", "rm", resourceName)
		default:
			return fmt.Errorf("unsupported Swarm resource type for deletion: %s", resourceType)
		}

	case cluster.TypeDocker:
		switch normalizedType {
		case "containers":
			return utils.RunCommand("docker", "rm", "-f", resourceName)
		case "images":
			return utils.RunCommand("docker", "rmi", resourceName)
		default:
			return fmt.Errorf("unsupported Docker resource type for deletion: %s", resourceType)
		}

	default:
		return fmt.Errorf("unsupported cluster type: %s", clusterType)
	}
}

// DescribeResource 查看资源详情 (保持与DescribeK8sResource相同风格)
func DescribeResource(clusterType cluster.ClusterType, resourceType, resourceName, namespace string) (string, error) {
	normalizedType, err := DetectResourceType(resourceType)
	if err != nil {
		return "", fmt.Errorf("invalid resource type: %w", err)
	}

	switch clusterType {
	case cluster.TypeK8s:
		args := []string{"describe", normalizedType, resourceName}
		if namespace != "" {
			args = append(args, "-n", namespace)
		}
		return utils.RunCommandWithOutput("kubectl", args...)

	case cluster.TypeSwarm:
		switch normalizedType {
		case "services":
			return utils.RunCommandWithOutput("docker", "service", "inspect", "--pretty", resourceName)
		case "nodes":
			return utils.RunCommandWithOutput("docker", "node", "inspect", "--pretty", resourceName)
		case "stacks":
			return utils.RunCommandWithOutput("docker", "stack", "ps", resourceName)
		default:
			return "", fmt.Errorf("unsupported Swarm resource type for describe: %s", resourceType)
		}

	case cluster.TypeDocker:
		switch normalizedType {
		case "containers":
			return utils.RunCommandWithOutput("docker", "inspect", "--format='{{json .}}'", resourceName)
		case "images":
			return utils.RunCommandWithOutput("docker", "image", "inspect", "--format='{{json .}}'", resourceName)
		default:
			return "", fmt.Errorf("unsupported Docker resource type for describe: %s", resourceType)
		}

	default:
		return "", fmt.Errorf("unsupported cluster type: %s", clusterType)
	}
}
