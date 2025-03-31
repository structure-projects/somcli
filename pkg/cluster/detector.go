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
package cluster

import (
	"strings"

	"github.com/structure-projects/somcli/pkg/utils"
)

// DefaultDetector 默认集群检测器
type DefaultDetector struct{}

// Detect 检测当前集群类型
func (d *DefaultDetector) Detect() (ClusterType, error) {
	// 1. 检测Kubernetes
	if d.isKubernetes() {
		return TypeK8s, nil
	}

	// 2. 检测Swarm
	if d.isSwarm() {
		return TypeSwarm, nil
	}

	// 3. 检测Docker
	if d.isDocker() {
		return TypeDocker, nil
	}

	return TypeNone, nil
}

// isKubernetes 检测是否是Kubernetes集群
func (d *DefaultDetector) isKubernetes() bool {
	if !utils.CommandExists("kubectl") {
		return false
	}

	// 检查是否能连接到集群
	output, err := utils.RunCommandWithOutput("kubectl", "cluster-info")
	if err != nil {
		return false
	}

	return strings.Contains(output, "is running at")
}

// isSwarm 检测是否是Swarm集群
func (d *DefaultDetector) isSwarm() bool {
	if !utils.CommandExists("docker") {
		return false
	}

	// 检查是否是swarm节点
	output, err := utils.RunCommandWithOutput("docker", "node", "ls")
	if err != nil {
		return false
	}

	return strings.Contains(output, "ID") && strings.Contains(output, "HOSTNAME")
}

// isDocker 检测是否安装了Docker
func (d *DefaultDetector) isDocker() bool {
	if !utils.CommandExists("docker") {
		return false
	}

	// 简单检查docker是否可用
	_, err := utils.RunCommandWithOutput("docker", "info")
	return err == nil
}

// DetectClusterType 检测当前集群类型（公共函数）
func DetectClusterType() ClusterType {
	detector := &DefaultDetector{}
	ct, _ := detector.Detect()
	return ct
}
