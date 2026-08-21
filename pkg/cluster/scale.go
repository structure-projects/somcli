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
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

// AddNode 往已有集群里加一台节点。
//
// 为什么不能用 cluster create 顶替：那条路会在第一台 master 上再跑一遍 kubeadm init，
// 撞上"配置文件已存在 / 端口已占用"直接失败。
//
// 要加的节点必须先写进配置文件（凭据、角色、IP 都在那儿），这里只按主机名指名。
func AddNode(configFile, clusterName, clusterType, host string, force bool) error {
	config, err := LoadConfig(configFile, clusterName, clusterType)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	switch config.Cluster.Type {
	case "k8s":
		return AddK8sNode(config, host, force)
	case "swarm":
		return AddSwarmNode(config, host, force)
	default:
		return fmt.Errorf("unsupported cluster type: %s", config.Cluster.Type)
	}
}

// RemoveNode 把一台节点从已有集群里摘掉。
func RemoveNode(configFile, clusterName, clusterType, host string, force bool) error {
	config, err := LoadConfig(configFile, clusterName, clusterType)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	switch config.Cluster.Type {
	case "k8s":
		return RemoveK8sNode(config, host, force)
	case "swarm":
		return RemoveSwarmNode(config, host, force)
	default:
		return fmt.Errorf("unsupported cluster type: %s", config.Cluster.Type)
	}
}

// AddK8sNode 装好依赖再把节点加入集群。
func AddK8sNode(config *types.ClusterConfig, host string, force bool) error {
	startTime := time.Now()
	utils.PrintBanner(fmt.Sprintf("正在向集群 %s 加入节点: %s", config.Cluster.Name, host))

	applyK8sDefaults(config)
	setK8sTemplateVars(config)

	target := findNodeByHost(config, host)
	if target == nil {
		return fmt.Errorf("配置里没有主机名为 %q 的节点，可用：%s\n"+
			"要加的节点得先写进配置的 nodes 里（用户名、密码、角色都在那儿）",
			host, strings.Join(nodeHosts(config), " / "))
	}

	if err := EnsureWorkDir(); err != nil {
		return fmt.Errorf("创建工作目录失败: %w", err)
	}

	// 整份配置还得是有效的：只加一台 master 也可能让集群从单 master 变成多 master，
	// 那时缺 controlPlaneEndpoint 就该在连节点之前拒绝。
	if err := validateK8sClusterConfig(config); err != nil {
		return fmt.Errorf("集群配置验证失败: %w", err)
	}

	first := findFirstMasterNode(config)
	if first == nil {
		return fmt.Errorf("配置中没有找到主节点，无从生成加入命令")
	}
	if first.IP == target.IP {
		return fmt.Errorf("%s 是配置里的第一台 master，集群就是从它装起来的，不能再加入一遍", host)
	}

	// 已经在集群里的节点直接拦住：不拦的话 kubeadm join 报的是
	// "/etc/kubernetes/kubelet.conf already exists"，指向的是文件而不是"这台早就加进来了"。
	inCluster, err := nodeInCluster(first, target.Host)
	if err != nil {
		return err
	}
	if inCluster {
		return fmt.Errorf("节点 %s 已经在集群里了；要重装它先 somcli cluster remove-node --node %s", host, host)
	}

	utils.PrintStage("== 节点准备 ==")
	if err := prepareK8sNodes(config, []types.RemoteNode{*target}); err != nil {
		return fmt.Errorf("节点准备失败: %w", err)
	}
	// 原有节点也要认得新来的主机名，否则它们解析不了这台。只重写 hosts 那一段，
	// 不重跑整套准备 —— 加一台机器不该动到别的机器上的 sysctl 与防火墙。
	for i := range config.Cluster.Nodes {
		node := config.Cluster.Nodes[i]
		if node.IP == target.IP {
			continue
		}
		if err := configureHostsFile(&node, hostsEntries(config)); err != nil {
			return fmt.Errorf("更新节点%s的hosts失败: %w", node.Host, err)
		}
	}
	utils.PrintSuccess("✓ 节点准备完成")

	utils.PrintStage("== 依赖安装 ==")
	// 只装到这一台上。网络插件不在这份清单里（见 defaultK8sResources），
	// 新节点上的 CNI 由集群里已有的 DaemonSet 铺。
	if err := installDependencies(config, []string{target.IP}, force); err != nil {
		return fmt.Errorf("依赖安装失败: %w", err)
	}
	utils.PrintSuccess("✓ 依赖安装完成")

	utils.PrintStage("== 加入集群 ==")
	if strings.EqualFold(target.Role, "master") {
		joinCmd, err := generateControlPlaneJoinCommand(first, target, config)
		if err != nil {
			return err
		}
		if output, err := utils.RunCommandOnNode(target, joinCmd); err != nil {
			return fmt.Errorf("主节点%s加入失败: %w\n输出: %s", target.Host, err, output)
		}
		if err := configureKubectl(target); err != nil {
			return fmt.Errorf("节点%s配置kubectl失败: %w", target.Host, err)
		}
	} else {
		joinCmd, err := generateJoinCommand(first, config)
		if err != nil {
			return err
		}
		if output, err := utils.RunCommandOnNode(target, " "+joinCmd); err != nil {
			return fmt.Errorf("工作节点%s加入失败: %w\n输出: %s", target.Host, err, output)
		}
	}

	// 加入命令退 0 不等于节点进了集群：kubelet 起不来时 Node 对象根本不会出现。
	if inCluster, err := nodeInCluster(first, target.Host); err != nil {
		return err
	} else if !inCluster {
		return fmt.Errorf("join 命令执行成功，但集群里查不到节点 %s；"+
			"到该节点上看 journalctl -u kubelet", target.Host)
	}

	utils.PrintSuccess("\n✓ 节点 %s 已加入集群，耗时: %v", host, time.Since(startTime).Round(time.Second))
	utils.PrintInfo("节点变 Ready 还要等 CNI 在这台上起来，用 kubectl get nodes -w 观察")
	return nil
}

// RemoveK8sNode 把一台节点从集群里摘掉：drain 疏散负载，reset 抹掉本机，最后删 Node 对象。
//
// 顺序按 kubeadm 的说法来：先 reset 再 delete node。反过来先删 Node 对象的话，
// 控制面节点上的 reset 少了从集群视图里退 etcd 成员的依据。
func RemoveK8sNode(config *types.ClusterConfig, host string, force bool) error {
	startTime := time.Now()
	utils.PrintBanner(fmt.Sprintf("正在从集群 %s 摘除节点: %s", config.Cluster.Name, host))

	applyK8sDefaults(config)

	target := findNodeByHost(config, host)
	if target == nil {
		return fmt.Errorf("配置里没有主机名为 %q 的节点，可用：%s\n"+
			"摘除也要读配置里的凭据才连得上这台机器",
			host, strings.Join(nodeHosts(config), " / "))
	}

	first := findFirstMasterNode(config)
	if first == nil {
		return fmt.Errorf("配置中没有找到主节点，无从操作集群")
	}
	if first.IP == target.IP {
		// 第一台 master 上有 admin.conf，摘了它剩下的步骤没地方执行；
		// 而且单 master 集群摘了它就什么都不剩了。
		return fmt.Errorf("%s 是配置里的第一台 master，摘掉它等于拆掉整个集群，"+
			"请用 somcli cluster remove", host)
	}

	if !force {
		if !utils.AskForConfirmation(fmt.Sprintf("确定要把节点 %s 从集群里摘除吗？", host)) {
			utils.PrintWarning("操作已取消")
			return fmt.Errorf("操作已取消")
		}
	}

	utils.PrintStage("== 疏散负载 ==")
	// --ignore-daemonsets：DaemonSet 的 Pod 疏散不走（控制器会立刻在原地重建），
	//   不加这个参数 drain 直接拒绝执行。
	// --delete-emptydir-data：用 emptyDir 的 Pod 同样会让 drain 拒绝执行；
	//   这台机器马上要被抹掉，那些数据本来也留不住。
	// --force：没有控制器管的裸 Pod 会让 drain 拒绝执行，它们随节点一起消失。
	drainCmd := fmt.Sprintf("KUBECONFIG=/etc/kubernetes/admin.conf kubectl drain %s "+
		"--ignore-daemonsets --delete-emptydir-data --force --timeout=5m", target.Host)
	if output, err := utils.RunCommandOnNode(first, drainCmd); err != nil {
		return fmt.Errorf("疏散节点%s失败: %w\n输出: %s\n"+
			"负载没疏散完就抹机器会直接打断上面的服务；"+
			"处理掉 drain 报的那些 Pod 之后再重试", target.Host, err, output)
	}
	utils.PrintSuccess("✓ 负载已疏散")

	utils.PrintStage("== 重置节点 ==")
	if err := resetK8sNode(target, config); err != nil {
		return err
	}
	utils.PrintSuccess("✓ 节点已重置")

	utils.PrintStage("== 删除节点对象 ==")
	// reset 时节点通常已经把自己删掉了，所以 not found 不算失败。
	deleteCmd := fmt.Sprintf("KUBECONFIG=/etc/kubernetes/admin.conf "+
		"kubectl delete node %s --ignore-not-found", target.Host)
	if output, err := utils.RunCommandOnNode(first, deleteCmd); err != nil {
		return fmt.Errorf("删除节点对象%s失败: %w\n输出: %s", target.Host, err, output)
	}

	if inCluster, err := nodeInCluster(first, target.Host); err != nil {
		return err
	} else if inCluster {
		return fmt.Errorf("删除之后集群里仍能查到节点 %s", target.Host)
	}

	utils.PrintSuccess("\n✓ 节点 %s 已摘除，耗时: %v", host, time.Since(startTime).Round(time.Second))
	utils.PrintInfo("配置文件里的这条 nodes 记录要自己删掉，否则下次 cluster remove 还会去连它")
	return nil
}

// AddSwarmNode 把一台节点加入已有的 swarm。
//
// 与 k8s 那侧同一个道理：重跑 cluster create 会在 manager 上再 docker swarm init 一次，
// 报的是"this node is already part of a swarm"。
func AddSwarmNode(config *types.ClusterConfig, host string, force bool) error {
	startTime := time.Now()
	utils.PrintBanner(fmt.Sprintf("正在向 swarm 集群 %s 加入节点: %s", config.Cluster.Name, host))

	target := findNodeByHost(config, host)
	if target == nil {
		return fmt.Errorf("配置里没有主机名为 %q 的节点，可用：%s\n"+
			"要加的节点得先写进配置的 nodes 里（用户名、密码、角色都在那儿）",
			host, strings.Join(nodeHosts(config), " / "))
	}

	if err := validateClusterConfig(config); err != nil {
		return fmt.Errorf("invalid cluster configuration: %w", err)
	}

	manager := findManagerNode(config)
	if manager == nil {
		return fmt.Errorf("配置中没有找到 manager 节点，无从取加入令牌")
	}
	if manager.Host == target.Host {
		return fmt.Errorf("%s 是配置里的第一台 manager，集群就是从它 init 起来的，不能再加入一遍", host)
	}

	// 已经在集群里的直接拦住：不拦的话 docker swarm join 报
	// "this node is already part of a swarm"，指向的是本机状态而不是"这台早就加进来了"。
	if in, err := swarmNodeInCluster(manager, target.Host); err != nil {
		return err
	} else if in {
		return fmt.Errorf("节点 %s 已经在 swarm 里了；要重来先 somcli cluster remove-node --node %s", host, host)
	}

	utils.PrintStage("== 节点准备 ==")
	// 只准备这一台（装 docker、开端口、写 hosts）。
	if err := prepareSwarmCluster(config, []types.RemoteNode{*target}, false); err != nil {
		return err
	}
	// 原有节点也要认得新来的主机名。只重写 hosts 那一段，不动别的。
	for i := range config.Cluster.Nodes {
		node := config.Cluster.Nodes[i]
		if node.IP == target.IP {
			continue
		}
		if err := configureHostsFile(&node, hostsEntries(config)); err != nil {
			return fmt.Errorf("更新节点%s的hosts失败: %w", node.Host, err)
		}
	}
	utils.PrintSuccess("✓ 节点准备完成")

	utils.PrintStage("== 加入集群 ==")
	// 现场取令牌，不读 create 时落下的那个文件：worker 令牌可以被轮换
	// （docker swarm join-token --rotate），文件里那条就失效了。
	tokens, err := extractSwarmJoinTokens(manager)
	if err != nil {
		return fmt.Errorf("获取 swarm 加入令牌失败: %w", err)
	}

	role := strings.ToLower(strings.TrimSpace(target.Role))
	joinCmd, ok := tokens[role]
	if !ok {
		return fmt.Errorf("节点 %s 的 role 是 %q，swarm 只认 manager / worker", host, target.Role)
	}
	// 令牌命令里的地址是 manager 自己打出来的，可能是主机名；换成 IP，
	// 免得新节点的 DNS 解析不了。
	joinCmd = strings.ReplaceAll(joinCmd, manager.Host, manager.IP)

	output, err := utils.RunCommandOnNode(target, joinCmd)
	if err != nil {
		return fmt.Errorf("节点%s加入失败: %w\n命令: %s\n输出: %s", target.Host, err, joinCmd, output)
	}
	if !strings.Contains(output, "This node joined a swarm") {
		return fmt.Errorf("节点%s可能没真的加入，输出: %s", target.Host, output)
	}

	// join 退 0 不等于 manager 那边看得到它。
	if in, err := swarmNodeInCluster(manager, target.Host); err != nil {
		return err
	} else if !in {
		return fmt.Errorf("join 命令执行成功，但 docker node ls 里查不到 %s", target.Host)
	}

	utils.PrintSuccess("\n✓ 节点 %s 已加入 swarm，耗时: %v", host, time.Since(startTime).Round(time.Second))
	return nil
}

// RemoveSwarmNode 把一台节点从 swarm 里摘掉：先腾空，再让它离开，最后删节点记录。
func RemoveSwarmNode(config *types.ClusterConfig, host string, force bool) error {
	startTime := time.Now()
	utils.PrintBanner(fmt.Sprintf("正在从 swarm 集群 %s 摘除节点: %s", config.Cluster.Name, host))

	target := findNodeByHost(config, host)
	if target == nil {
		return fmt.Errorf("配置里没有主机名为 %q 的节点，可用：%s\n"+
			"摘除也要读配置里的凭据才连得上这台机器",
			host, strings.Join(nodeHosts(config), " / "))
	}

	manager := findManagerNode(config)
	if manager == nil {
		return fmt.Errorf("配置中没有找到 manager 节点，无从操作集群")
	}
	if manager.Host == target.Host {
		// 摘除的每一步都在 manager 上执行；它自己走了，剩下的步骤没地方跑。
		return fmt.Errorf("%s 是配置里的第一台 manager，摘掉它等于拆掉整个集群，"+
			"请用 somcli cluster remove", host)
	}

	if !force {
		if !utils.AskForConfirmation(fmt.Sprintf("确定要把节点 %s 从 swarm 里摘除吗？", host)) {
			utils.PrintWarning("操作已取消")
			return fmt.Errorf("操作已取消")
		}
	}

	utils.PrintStage("== 腾空节点 ==")
	// swarm 版的 drain：把这台标成 drain，上面的任务会被调度到别处再让它离开。
	// 少了这一步，服务的副本随节点一起消失，要等 swarm 自己发现节点掉线才补 ——
	// 中间那段时间是实打实的服务缺口。
	drainCmd := fmt.Sprintf("docker node update --availability drain %s", target.Host)
	if output, err := utils.RunCommandOnNode(manager, drainCmd); err != nil {
		return fmt.Errorf("腾空节点%s失败: %w\n输出: %s", target.Host, err, output)
	}
	utils.PrintSuccess("✓ 节点已标为 drain")

	utils.PrintStage("== 离开集群 ==")
	// manager 离开要 --force：swarm 怕你把 raft 法定人数拆了，默认不让走。
	leaveCmd := "docker swarm leave"
	if strings.EqualFold(target.Role, "manager") {
		leaveCmd += " --force"
	}
	if output, err := utils.RunCommandOnNode(target, leaveCmd); err != nil {
		return fmt.Errorf("节点%s离开swarm失败: %w\n输出: %s", target.Host, err, output)
	}

	utils.PrintStage("== 删除节点记录 ==")
	// 离开之后节点在 manager 眼里是 Down，记录还在，要显式删掉。
	// --force 是因为它可能还没被标成 Down（离开的状态同步要一会儿）。
	rmCmd := fmt.Sprintf("docker node rm --force %s", target.Host)
	if output, err := utils.RunCommandOnNode(manager, rmCmd); err != nil {
		return fmt.Errorf("删除节点记录%s失败: %w\n输出: %s", target.Host, err, output)
	}

	if in, err := swarmNodeInCluster(manager, target.Host); err != nil {
		return err
	} else if in {
		return fmt.Errorf("删除之后 docker node ls 里仍能查到 %s", target.Host)
	}

	utils.PrintSuccess("\n✓ 节点 %s 已摘除，耗时: %v", host, time.Since(startTime).Round(time.Second))
	utils.PrintInfo("配置文件里的这条 nodes 记录要自己删掉，否则下次 cluster remove 还会去连它")
	return nil
}

// swarmNodeInCluster 问 manager 那边有没有这台节点。
//
// 逐行精确比对，不用 grep：k8s-worker 与 k8s-worker2 会互相蒙住。
func swarmNodeInCluster(manager *types.RemoteNode, host string) (bool, error) {
	const cmd = "docker node ls --format '{{.Hostname}}'"
	output, err := utils.RunCommandOnNode(manager, cmd)
	if err != nil {
		// 连不上、或者这台根本不是 manager，都不是"节点不在集群里"。分不清就别猜。
		return false, fmt.Errorf("在 manager %s 上查询节点%s失败: %w\n输出: %s",
			manager.Host, host, err, output)
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == host {
			return true, nil
		}
	}
	return false, nil
}

// nodeInCluster 问集群里有没有这个 Node 对象。
//
// 用 `get node <名字>` 而不是 `get nodes | grep`：后者会被别的同前缀主机名蒙住
// （k8s-worker 与 k8s-worker2）。
func nodeInCluster(master *types.RemoteNode, host string) (bool, error) {
	cmd := fmt.Sprintf("KUBECONFIG=/etc/kubernetes/admin.conf "+
		"kubectl get node %s -o name 2>/dev/null || true", host)
	output, err := utils.RunCommandOnNode(master, cmd)
	if err != nil {
		// 连不上或者 kubectl 不在，都不是"节点不在集群里"。分不清就别猜。
		return false, fmt.Errorf("在主节点%s上查询节点%s失败: %w\n输出: %s", master.Host, host, err, output)
	}
	return strings.Contains(output, "node/"+host), nil
}

// findNodeByHost 按主机名找配置里的节点。
func findNodeByHost(config *types.ClusterConfig, host string) *types.RemoteNode {
	host = strings.TrimSpace(host)
	for i := range config.Cluster.Nodes {
		if config.Cluster.Nodes[i].Host == host {
			return &config.Cluster.Nodes[i]
		}
	}
	return nil
}

// nodeHosts 列出配置里的主机名，排序后给报错用 —— 名字写错时得能看到有哪些可选。
func nodeHosts(config *types.ClusterConfig) []string {
	hosts := make([]string, 0, len(config.Cluster.Nodes))
	for i := range config.Cluster.Nodes {
		hosts = append(hosts, config.Cluster.Nodes[i].Host)
	}
	sort.Strings(hosts)
	return hosts
}
