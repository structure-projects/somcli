package cluster

// 集群类型常量
const (
	TypeK8s    = "k8s"    // Kubernetes集群
	TypeSwarm  = "swarm"  // Docker Swarm集群
	TypeDocker = "docker" // 单机Docker
	TypeNone   = "none"   // 无集群
)

// ClusterType 表示集群类型的字符串别名
type ClusterType string

// 集群类型检测器接口
type Detector interface {
	Detect() (ClusterType, error)
}
