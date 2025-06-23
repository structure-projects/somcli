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
	"strings"

	"github.com/structure-projects/somcli/pkg/types"
)

// ResourceMapper 资源类型映射
var ResourceMapper = map[string]string{
	// Kubernetes 资源类型
	"pod":          "pods",
	"pods":         "pods",
	"po":           "pods",
	"deployment":   "deployments",
	"deployments":  "deployments",
	"deploy":       "deployments",
	"service":      "services",
	"services":     "services",
	"svc":          "services",
	"statefulset":  "statefulsets",
	"statefulsets": "statefulsets",
	"sts":          "statefulsets",
	"s":            "statefulsets",
	"node":         "nodes",
	"nodes":        "nodes",
	"no":           "nodes",

	// Docker/Swarm 资源类型
	"container":  "containers",
	"containers": "containers",
	"ct":         "containers",
	"stack":      "stacks",
	"stacks":     "stacks",
}

// DetectResourceType 检测并规范化资源类型
func DetectResourceType(input string) (string, error) {
	if res, ok := ResourceMapper[strings.ToLower(input)]; ok {
		return res, nil
	}
	return "", fmt.Errorf("unsupported resource type: %s", input)
}

// GetClusterConfig 获取当前集群配置
func GetClusterConfig() (*types.ClusterConfig, error) {
	// 实现从本地缓存或配置文件中加载集群配置
	return nil, nil
}

// 添加Describe接口
type ResourceDescriber interface {
	Describe(resourceType, name, namespace string) (string, error)
}
