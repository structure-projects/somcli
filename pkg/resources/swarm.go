package resources

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/structure-projects/somcli/pkg/utils"
	"gopkg.in/yaml.v2"
)

// GetSwarmResources 获取Swarm资源
func GetSwarmResources(resourceType, namespace string) (string, error) {
	switch resourceType {
	case "services":
		return utils.RunCommandWithOutput("docker", "service", "ls")
	case "nodes":
		return utils.RunCommandWithOutput("docker", "node", "ls")
	case "stacks":
		return utils.RunCommandWithOutput("docker", "stack", "ls")
	default:
		return "", fmt.Errorf("unsupported Swarm resource type: %s", resourceType)
	}
}

// ApplySwarmResources 应用Swarm配置
func ApplySwarmResources(file string) error {
	// 转换为docker stack deploy格式
	stackName, composeFile, err := convertToStackFormat(file)
	if err != nil {
		return err
	}

	return utils.RunCommand("docker", "stack", "deploy", "-c", composeFile, stackName)
}

// DeleteSwarmResource 删除Swarm资源
func DeleteSwarmResource(resourceType, name string) error {
	switch resourceType {
	case "services":
		return utils.RunCommand("docker", "service", "rm", name)
	case "stacks":
		return utils.RunCommand("docker", "stack", "rm", name)
	default:
		return fmt.Errorf("unsupported Swarm resource type for deletion: %s", resourceType)
	}
}

// convertToStackFormat 将资源文件转换为docker stack格式
func convertToStackFormat(inputFile string) (string, string, error) {
	// 读取输入文件
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to read input file: %w", err)
	}

	// 解析为基本结构获取stack名称
	var manifest struct {
		Stack string `yaml:"stack"`
	}
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return "", "", fmt.Errorf("failed to parse YAML: %w", err)
	}

	if manifest.Stack == "" {
		manifest.Stack = filepath.Base(inputFile[:len(inputFile)-len(filepath.Ext(inputFile))])
	}

	// 保存为临时文件
	tempFile := filepath.Join(utils.GetWorkDir(), "docker-stack-"+manifest.Stack+".yaml")
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return "", "", fmt.Errorf("failed to write temp file: %w", err)
	}

	return manifest.Stack, tempFile, nil
}

// 添加Swarm描述功能
func DescribeSwarmResource(resourceType, name string) (string, error) {
	switch resourceType {
	case "services":
		return utils.RunCommandWithOutput("docker", "service", "inspect", "--pretty", name)
	case "nodes":
		return utils.RunCommandWithOutput("docker", "node", "inspect", "--pretty", name)
	case "stacks":
		// Docker stack没有直接describe命令，获取所有服务
		return utils.RunCommandWithOutput("docker", "stack", "services", name)
	default:
		return "", fmt.Errorf("unsupported Swarm resource type for describe: %s", resourceType)
	}
}
