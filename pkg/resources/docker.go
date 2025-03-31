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
package resources

import (
	"fmt"

	"github.com/structure-projects/somcli/pkg/utils"
)

// GetDockerResources 获取Docker资源
func GetDockerResources(resourceType string) (string, error) {
	switch resourceType {
	case "containers":
		return utils.RunCommandWithOutput("docker", "ps", "-a")
	case "images":
		return utils.RunCommandWithOutput("docker", "images")
	default:
		return "", fmt.Errorf("unsupported Docker resource type: %s", resourceType)
	}
}

// ApplyDockerResources 应用Docker配置
func ApplyDockerResources(file string) error {
	return utils.RunCommand("docker-compose", "-f", file, "up", "-d")
}

// DeleteDockerResource 删除Docker资源
func DeleteDockerResource(resourceType, name string) error {
	switch resourceType {
	case "containers":
		return utils.RunCommand("docker", "rm", "-f", name)
	case "images":
		return utils.RunCommand("docker", "rmi", name)
	default:
		return fmt.Errorf("unsupported Docker resource type for deletion: %s", resourceType)
	}
}

// 添加Docker描述功能
func DescribeDockerResource(resourceType, name string) (string, error) {
	switch resourceType {
	case "containers":
		return utils.RunCommandWithOutput("docker", "inspect", "--format='{{json .}}'", name)
	case "images":
		return utils.RunCommandWithOutput("docker", "image", "inspect", "--format='{{json .}}'", name)
	default:
		return "", fmt.Errorf("unsupported Docker resource type for describe: %s", resourceType)
	}
}
