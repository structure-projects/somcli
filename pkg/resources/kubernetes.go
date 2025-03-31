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

// GetK8sResources 获取Kubernetes资源
func GetK8sResources(resourceType, namespace string, allNamespaces bool, outputFormat string) (string, error) {
	args := []string{"get", resourceType}

	if allNamespaces {
		args = append(args, "-A")
	} else if namespace != "" {
		args = append(args, "-n", namespace)
	}

	if outputFormat != "" {
		args = append(args, "-o", outputFormat)
	}

	output, err := utils.RunCommandWithOutput("kubectl", args...)
	if err != nil {
		return "", fmt.Errorf("kubectl command failed: %w", err)
	}

	return output, nil
}

// ApplyK8sResources 应用Kubernetes配置
func ApplyK8sResources(file string) error {
	return utils.RunCommand("kubectl", "apply", "-f", file)
}

// DeleteK8sResource 删除Kubernetes资源
func DeleteK8sResource(resourceType, name, namespace string) error {
	args := []string{"delete", resourceType, name}

	if namespace != "" {
		args = append(args, "-n", namespace)
	}

	return utils.RunCommand("kubectl", args...)
}

// 添加Kubernetes描述功能
func DescribeK8sResource(resourceType, name, namespace string) (string, error) {
	args := []string{"describe", resourceType, name}

	if namespace != "" {
		args = append(args, "-n", namespace)
	}

	output, err := utils.RunCommandWithOutput("kubectl", args...)
	if err != nil {
		return "", fmt.Errorf("kubectl describe failed: %w", err)
	}

	return output, nil
}
