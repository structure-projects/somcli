package resources

import (
	"fmt"
	"strings"

	"github.com/structure-projects/somcli/pkg/cluster"
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
func GetClusterConfig() (*cluster.ClusterConfig, error) {
	// 实现从本地缓存或配置文件中加载集群配置
	return nil, nil
}

// 添加Describe接口
type ResourceDescriber interface {
	Describe(resourceType, name, namespace string) (string, error)
}
