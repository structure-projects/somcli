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
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/structure-projects/somcli/pkg/installer"
	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

// 组件的默认版本。
//
// 必须有默认值：配置里不写 containerdVersion 时，URL 会拼成
// .../download/v/containerd--linux-amd64.tar.gz —— 下载 404，而报错只说"下载失败"，
// 用户根本看不出是自己少写了一个键。
//
// 与 configs/k8s/*.yaml 里的 version: 是同一批值，两处都留着是因为职责不同：
// 清单文件的 version 让它能被 somcli install -f 单独喂进去，这里的默认值负责
// 在集群配置缺键时把日志打清楚（"xxx 未配置，使用默认值"）。
const (
	defaultContainerdVersion = "1.7.22"
	defaultRuncVersion       = "1.1.14"
	defaultCniPluginsVersion = "1.5.1"
	defaultDockerVersion     = "20.10.24"

	cniFlannel            = "flannel"
	cniCalico             = "calico"
	defaultFlannelVersion = "0.25.6"
	defaultCalicoVersion  = "3.28.2"
)

// applyK8sDefaults 把配置里没写的组件版本补齐，并把补出来的值打出来。
//
// 打出来是必要的：装上的东西与配置里写的不完全一致时，用户得能在日志里看到
// 到底装的是哪个版本，否则出问题时无从对照。
func applyK8sDefaults(config *types.ClusterConfig) {
	k8s := &config.Cluster.K8sConfig

	if strings.TrimSpace(k8s.ContainerRuntime) == "" {
		k8s.ContainerRuntime = "containerd"
		utils.PrintInfo("  containerRuntime 未配置，使用默认值: %s", k8s.ContainerRuntime)
	}
	if strings.TrimSpace(k8s.Cni) == "" {
		k8s.Cni = cniFlannel
		utils.PrintInfo("  cni 未配置，使用默认值: %s", k8s.Cni)
	}

	defaults := []struct {
		key   string
		field *string
		value string
	}{
		{"containerdVersion", &k8s.ContainerdVersion, defaultContainerdVersion},
		{"runcVersion", &k8s.RuncVersion, defaultRuncVersion},
		{"cniPluginsVersion", &k8s.CniPluginsVersion, defaultCniPluginsVersion},
		{"dockerVersion", &k8s.DockerVersion, defaultDockerVersion},
		{"cniVersion", &k8s.CniVersion, defaultCNIVersion(k8s.Cni)},
	}
	for _, d := range defaults {
		if strings.TrimSpace(*d.field) == "" {
			*d.field = d.value
			utils.PrintInfo("  %s 未配置，使用默认值: %s", d.key, d.value)
		}
	}
}

// defaultCNIVersion 返回该网络插件的默认版本。
func defaultCNIVersion(cni string) string {
	if strings.EqualFold(strings.TrimSpace(cni), cniCalico) {
		return defaultCalicoVersion
	}
	return defaultFlannelVersion
}

// CreateK8sCluster 创建Kubernetes集群
func CreateK8sCluster(config *types.ClusterConfig, force bool, skipPrecheck bool) error {
	startTime := time.Now()
	utils.PrintBanner(fmt.Sprintf("正在创建Kubernetes集群: %s", config.Cluster.Name))
	utils.PrintInfo("开始时间: %s", startTime.Format("2006-01-02 15:04:05"))
	utils.PrintInfo("集群配置详情:")
	applyK8sDefaults(config)
	// 补完默认值再交给模板：configs/k8s 里的清单要拿这些值渲染，
	// 顺序颠倒的话 cni 之类的键会以空串下去。
	setK8sTemplateVars(config)
	utils.PrintInfo("  集群名称: %s", config.Cluster.Name)
	utils.PrintInfo("  Kubernetes版本: %s", config.Cluster.K8sConfig.Version)
	utils.PrintInfo("  Pod网络CIDR: %s", config.Cluster.K8sConfig.PodNetworkCidr)
	utils.PrintInfo("  服务CIDR: %s", config.Cluster.K8sConfig.ServiceCidr)
	utils.PrintInfo("  容器运行时: %s", config.Cluster.K8sConfig.ContainerRuntime)
	utils.PrintInfo("  网络插件: %s %s", config.Cluster.K8sConfig.Cni, config.Cluster.K8sConfig.CniVersion)
	utils.PrintInfo("  Docker版本: %s", config.Cluster.K8sConfig.DockerVersion)
	utils.PrintInfo("  Containerd版本: %s", config.Cluster.K8sConfig.ContainerdVersion)

	// 1. 准备阶段
	utils.PrintStage("== 集群准备阶段 ==")
	if err := prepareK8sCluster(config, skipPrecheck); err != nil {
		utils.PrintError("集群准备失败: %v", err)
		return fmt.Errorf("集群准备失败: %w", err)
	}
	utils.PrintSuccess("✓ 集群准备完成")

	// 2. 依赖安装阶段
	if err := installDependencies(config, getAllNodesIP(config), force); err != nil {
		utils.PrintError("依赖安装失败: %v", err)
		return fmt.Errorf("依赖安装失败: %w", err)
	}
	utils.PrintSuccess("✓ 依赖安装完成")

	// 3. 主节点初始化
	utils.PrintStage("== 主节点初始化 ==")
	masterNode := findFirstMasterNode(config)
	if masterNode == nil {
		err := fmt.Errorf("配置中没有找到主节点")
		utils.PrintError("未找到主节点: %v", err)
		return err
	}
	utils.PrintInfo("已选择主节点: %s (%s)", masterNode.Host, masterNode.IP)

	if err := initK8sMaster(masterNode, config); err != nil {
		utils.PrintError("主节点初始化失败: %v", err)
		return fmt.Errorf("主节点初始化失败: %w", err)
	}
	utils.PrintSuccess("✓ 主节点初始化完成")

	// 4. 部署网络插件
	//
	// 必须在 worker 加入之前：没有 CNI 时 kubelet 报 NetworkReady=false，节点永久 NotReady，
	// 先加 worker 只是多几个 NotReady 的节点。
	utils.PrintStage("== 网络插件部署 ==")
	if err := deployCNI(masterNode, config); err != nil {
		utils.PrintError("网络插件部署失败: %v", err)
		return fmt.Errorf("网络插件部署失败: %w", err)
	}
	utils.PrintSuccess("✓ 网络插件部署完成")

	// 5. 其他主节点加入
	//
	// 排在 CNI 之后、worker 之前：控制面节点同样要等 CNI 才 Ready，
	// 而 worker 加入时若控制面还没齐，kube-proxy 之类的 DaemonSet 会先在残缺的
	// 控制面上铺一遍，后加进来的 master 上就少了它们。
	utils.PrintStage("== 其他主节点加入 ==")
	if err := joinMasterNodes(config); err != nil {
		utils.PrintError("主节点加入失败: %v", err)
		return fmt.Errorf("主节点加入失败: %w", err)
	}
	utils.PrintSuccess("✓ 其他主节点加入完成")

	// 6. 工作节点加入
	utils.PrintStage("== 工作节点加入 ==")
	if err := joinWorkerNodes(config); err != nil {
		utils.PrintError("工作节点加入失败: %v", err)
		return fmt.Errorf("工作节点加入失败: %w", err)
	}
	utils.PrintSuccess("✓ 工作节点加入完成")

	// 7. 集群信息展示
	utils.PrintStage("== 集群信息展示 ==")
	if err := printK8sClusterInfo(config, masterNode); err != nil {
		utils.PrintError("集群信息展示失败: %v", err)
		return fmt.Errorf("集群信息展示失败: %w", err)
	}

	duration := time.Since(startTime)
	utils.PrintSuccess("\n✓ Kubernetes集群 '%s' 创建成功!", config.Cluster.Name)
	utils.PrintInfo("总执行时间: %v", duration.Round(time.Second))

	return nil
}

// installDependencies 按 k8sConfig.resources 逐个装。
//
// 装什么全在 configs/k8s/*.yaml 里，这里只做编排：定顺序、定节点、定版本，
// 剩下的下载、分发、幂等、失败传播都交给通用引擎。
// 外置之前这些内容是本文件里的 Go 字面量 —— 换个版本、加一条 sed 都要改代码重编译，
// 而"这套集群到底装了什么"只能靠读 Go 代码回答。
//
// force 来自 cluster create --force，含义与 install --force 一致：不看幂等状态重装一遍。
// 这个标志此前一路传进来却没有任何消费点，加了与不加完全一样。
//
// hosts 是这批资源要装到哪几台上。扩容时只装新来的那一台 —— 传全量的话，
// 已在集群里的节点会被 alwaysApply 那几条重跑一遍（base-dependencies 会 swapoff、
// 改 sysctl），加一台机器不该动到其他机器。
func installDependencies(config *types.ClusterConfig, hosts []string, force bool) error {
	utils.PrintInfo("正在准备安装Kubernetes %s...", config.Cluster.K8sConfig.Version)

	catalog, err := loadK8sCatalog()
	if err != nil {
		return err
	}

	names := k8sResourceNames(config)
	utils.PrintInfo("待安装资源: %s", strings.Join(names, " -> "))

	engine := installer.NewInstaller()
	for _, name := range names {
		res, err := resourceFromCatalog(catalog, name, k8sVersionFor(name, config.Cluster.K8sConfig), hosts)
		if err != nil {
			return err
		}

		utils.PrintInfo("正在安装 %s %s...", res.Name, res.Version)
		if err := engine.Install(res, force || alwaysApply[name]); err != nil {
			return fmt.Errorf("安装 %s 失败: %w", name, err)
		}
	}

	return nil
}

// initK8sMaster 初始化 Kubernetes 主节点
func initK8sMaster(node *types.RemoteNode, config *types.ClusterConfig) error {
	// 构建初始化命令
	initCmd := fmt.Sprintf(
		"kubeadm init --kubernetes-version=%s --apiserver-advertise-address=%s --pod-network-cidr=%s --service-cidr=%s",
		config.Cluster.K8sConfig.Version,
		node.IP,
		config.Cluster.K8sConfig.PodNetworkCidr,
		config.Cluster.K8sConfig.ServiceCidr,
	)

	// 有稳定入口就交给 kubeadm：证书 SAN 与 admin.conf 都会指向它而不是这台机器的 IP。
	if endpoint := strings.TrimSpace(config.Cluster.K8sConfig.ControlPlaneEndpoint); endpoint != "" {
		initCmd += fmt.Sprintf(" --control-plane-endpoint=%s", endpoint)
	}

	// 多 master 才加 --upload-certs：它把控制面证书存成 kube-system 下的一个 Secret，
	// 后面的 master 凭 certificate-key 取回来，不必手工拷 /etc/kubernetes/pki。
	// 单 master 不加，免得白留一份带证书的 Secret（两小时后自动过期，但也没必要有）。
	if len(findMasterNodes(config)) > 1 {
		initCmd += " --upload-certs"
	}

	initCmd += " --cri-socket " + criSocket(config.Cluster.K8sConfig.ContainerRuntime)

	// 添加镜像仓库配置
	if config.Cluster.K8sConfig.ImageRepository != "" {
		initCmd += fmt.Sprintf(" --image-repository=%s", config.Cluster.K8sConfig.ImageRepository)
	}

	utils.PrintInfo("正在使用以下命令初始化主节点:")
	utils.PrintInfo("  %s", initCmd)

	startTime := time.Now()
	output, err := utils.RunCommandOnNode(node, initCmd)
	if err != nil {
		utils.PrintError("主节点初始化失败: %v", err)
		utils.PrintDebug("命令输出:\n%s", output)
		return fmt.Errorf("主节点初始化失败: %w\n输出: %s", err, output)
	}

	// D4：不再从 init 输出里刮 join 命令。kubeadm 把它打成两行（第二行是续行，
	// 带着 --discovery-token-ca-cert-hash），按行取只会拿到半条命令，
	// 而半条命令跑起来是"缺少 CA hash"这种指不到根因的错。
	// join 时现场用 kubeadm token create --print-join-command 生成，见 joinWorkerNodes。

	utils.PrintInfo("正在配置kubectl...")
	if err := configureKubectl(node); err != nil {
		return err
	}

	duration := time.Since(startTime)
	utils.PrintSuccess("✓ 主节点初始化完成，耗时: %v", duration.Round(time.Second))
	return nil
}

// configureKubectl 把 admin.conf 放到目标节点的 $HOME/.kube/config。
//
// 每个控制面节点都要做一遍：登上任意一台 master 敲 kubectl 都该能用，
// 而 admin.conf 是 kubeadm 各自在本机生成的，不会自己进 $HOME。
func configureKubectl(node *types.RemoteNode) error {
	for _, cmd := range []string{
		"mkdir -p $HOME/.kube",
		// -f 而不是 -i：-i 在没有终端时读到 EOF 就放弃覆盖，于是重装出来的集群
		// 用的还是上一次的 kubeconfig，证书早就不匹配了。
		"cp -f /etc/kubernetes/admin.conf $HOME/.kube/config",
		"chown $(id -u):$(id -g) $HOME/.kube/config",
	} {
		if output, err := utils.RunCommandOnNode(node, cmd); err != nil {
			utils.PrintError("命令执行失败: %s: %v", cmd, err)
			return fmt.Errorf("在 %s 上配置 kubectl 失败: %w\n输出: %s", node.Host, err, output)
		}
	}
	return nil
}

// getAllNodesIP 获取所有节点IP
func getAllNodesIP(config *types.ClusterConfig) []string {
	hosts := []string{}
	for _, node := range config.Cluster.Nodes {
		hosts = append(hosts, node.IP)
	}
	return hosts
}

// prepareK8sCluster 准备Kubernetes集群
func prepareK8sCluster(config *types.ClusterConfig, skipPrecheck bool) error {
	utils.PrintInfo("正在创建工作目录...")
	if err := EnsureWorkDir(); err != nil {
		utils.PrintError("创建工作目录失败: %v", err)
		return fmt.Errorf("创建工作目录失败: %w", err)
	}

	if skipPrecheck {
		utils.PrintWarning("⚠ 跳过环境预检查，这可能导致安装失败")
		return nil
	}

	utils.PrintInfo("正在验证集群配置...")
	if err := validateK8sClusterConfig(config); err != nil {
		utils.PrintError("集群配置验证失败: %v", err)
		return fmt.Errorf("集群配置验证失败: %w", err)
	}
	utils.PrintSuccess("✓ 集群配置验证通过")

	utils.PrintInfo("集群节点配置:")
	for i, node := range config.Cluster.Nodes {
		utils.PrintInfo("  节点%d: 主机名=%s, IP=%s, 角色=%s", i+1, node.Host, node.IP, node.Role)
	}

	utils.PrintInfo("正在准备节点...")
	if err := prepareK8sNodes(config, config.Cluster.Nodes); err != nil {
		utils.PrintError("节点准备失败: %v", err)
		return fmt.Errorf("节点准备失败: %w", err)
	}

	return nil
}

// hostsEntries 拼出 /etc/hosts 里那一段。取的是配置里的全部节点，
// 不是"这次要动的节点"：新加的一台也得认得原有的主机名。
func hostsEntries(config *types.ClusterConfig) string {
	var b strings.Builder
	for _, node := range config.Cluster.Nodes {
		b.WriteString(fmt.Sprintf("%s\t%s\n", node.IP, node.Host))
	}
	return b.String()
}

// prepareK8sNodes 准备指定的这几台节点。扩容时只传新来的那一台。
func prepareK8sNodes(config *types.ClusterConfig, nodes []types.RemoteNode) error {
	entries := hostsEntries(config)

	for _, node := range nodes {
		utils.PrintStage(fmt.Sprintf("准备节点: %s (%s)", node.Host, node.IP))
		startTime := time.Now()

		utils.PrintInfo("正在检查操作系统...")
		if err := checkAndConfigureOS(&node); err != nil {
			utils.PrintError("操作系统配置失败: %v", err)
			return fmt.Errorf("节点%s操作系统配置失败: %w", node.Host, err)
		}

		utils.PrintInfo("正在配置hosts文件...")
		if err := configureHostsFile(&node, entries); err != nil {
			utils.PrintError("hosts配置失败: %v", err)
			return fmt.Errorf("节点%s hosts配置失败: %w", node.Host, err)
		}

		// F10：这个函数一直存在但从来没被调用过。防火墙开着的话，
		// worker 连不上 6443、kubelet 连不上 10250，表现是节点 NotReady 而不是"被墙了"。
		// 完整的发行版适配在 M3，这里只把已有的能力接进流程。
		utils.PrintInfo("正在配置防火墙...")
		if err := configureFirewall(&node); err != nil {
			utils.PrintError("防火墙配置失败: %v", err)
			return fmt.Errorf("节点%s防火墙配置失败: %w", node.Host, err)
		}

		duration := time.Since(startTime)
		utils.PrintSuccess("✓ 节点%s准备完成，耗时: %v", node.Host, duration.Round(time.Second))
	}

	return nil
}

// checkAndConfigureOS 检查并配置操作系统
func checkAndConfigureOS(node *types.RemoteNode) error {
	utils.PrintInfo("正在检查操作系统类型...")
	osType, err := utils.RunCommandOnNode(node, "uname -s")
	if err != nil {
		return fmt.Errorf("检查OS类型失败: %w", err)
	}
	if !strings.Contains(strings.ToLower(osType), "linux") {
		return fmt.Errorf("不支持的OS类型: %s，仅支持Linux", osType)
	}
	utils.PrintInfo("操作系统类型: %s", strings.TrimSpace(osType))

	utils.PrintInfo("正在检查CPU架构...")
	arch, err := utils.RunCommandOnNode(node, "uname -m")
	if err != nil {
		return fmt.Errorf("检查CPU架构失败: %w", err)
	}
	// k8s 官方为 amd64 与 arm64 都发布二进制（configs/k8s 里的 URL 已用 {{.Arch}}
	// 参数化），所以两档都放行。别的架构（386 / armv7 / ppc64le）上游不全，
	// 提前拒绝，别等下载 404 或二进制跑不起来才报一个指不到根因的错。
	// 用精确匹配而不是 Contains：Contains 会把 "x86_64 haswell" 这类异常输出也放过。
	switch strings.ToLower(strings.TrimSpace(arch)) {
	case "x86_64", "amd64", "aarch64", "arm64":
	default:
		return fmt.Errorf("不支持的CPU架构: %s，仅支持 x86_64/amd64 与 aarch64/arm64（k8s 集群安装在 arm64 上为实验性支持）", arch)
	}
	utils.PrintInfo("CPU架构: %s", strings.TrimSpace(arch))

	utils.PrintInfo("正在检查内存大小...")
	memInfo, err := utils.RunCommandOnNode(node, "free -b")
	if err != nil {
		return fmt.Errorf("检查内存大小失败: %w", err)
	}

	lines := strings.Split(memInfo, "\n")
	if len(lines) < 2 {
		return fmt.Errorf("内存信息格式无效")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		return fmt.Errorf("内存信息格式无效")
	}

	totalMem, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return fmt.Errorf("解析内存大小失败: %w", err)
	}

	minMem := int64(2 * 1024 * 1024 * 1024) // 2GB
	if totalMem < minMem {
		return fmt.Errorf("内存不足: %d字节(最低要求: %d字节)", totalMem, minMem)
	}
	utils.PrintInfo("总内存: %.2fGB", float64(totalMem)/float64(1024*1024*1024))

	utils.PrintInfo("正在禁用交换分区...")
	if _, err := utils.RunCommandOnNode(node, " swapoff -a"); err != nil {
		return fmt.Errorf("禁用交换分区失败: %w", err)
	}

	if _, err := utils.RunCommandOnNode(node, " sed -i '/ swap / s/^/#/' /etc/fstab"); err != nil {
		return fmt.Errorf("永久禁用交换分区失败: %w", err)
	}

	return nil
}

// validateK8sClusterConfig 验证 Kubernetes 集群配置
func validateK8sClusterConfig(config *types.ClusterConfig) error {
	if config.Cluster.Name == "" {
		return fmt.Errorf("集群名称不能为空")
	}

	if len(config.Cluster.Nodes) == 0 {
		return fmt.Errorf("集群配置中没有定义节点")
	}

	masterCount := 0
	for _, node := range config.Cluster.Nodes {
		if !utils.IsValidIP(node.IP) {
			return fmt.Errorf("节点%s的IP地址格式无效: %s", node.Host, node.IP)
		}

		if strings.ToLower(node.Role) == "master" {
			masterCount++
		}
	}

	if masterCount == 0 {
		return fmt.Errorf("至少需要一个主节点")
	}

	// 多 master 必须有一个不随单机存亡的 apiserver 入口，否则装出来的只会是个单点集群。
	// 这件事在配置层面就能判定，必须在连节点之前拒绝：连上去之后再说不行，节点已经被改过了。
	if masterCount > 1 && strings.TrimSpace(config.Cluster.K8sConfig.ControlPlaneEndpoint) == "" {
		return fmt.Errorf("配置了 %d 个 master 却没有 controlPlaneEndpoint：\n"+
			"多 master 需要一个稳定的 apiserver 入口（VIP 或负载均衡），"+
			"证书与 kubeconfig 都要指向它；缺了它另外几个 master 无从加入，"+
			"装出来的仍是单点集群。\n"+
			"请在 k8sConfig 下补 controlPlaneEndpoint: \"<VIP 或 LB 地址>:6443\"，"+
			"或只保留一个 master", masterCount)
	}

	if config.Cluster.K8sConfig.PodNetworkCidr == "" {
		return fmt.Errorf("Pod网络CIDR不能为空")
	}

	if config.Cluster.K8sConfig.ServiceCidr == "" {
		return fmt.Errorf("服务CIDR不能为空")
	}

	// 网络插件的取值在连节点之前就该判定：认不出来的话装完 kubeadm 才发现没 CNI 可装，
	// 而节点已经被改过一遍了。留空由 applyK8sDefaults 填成 flannel，所以这里只看非空值。
	if cni := strings.ToLower(strings.TrimSpace(config.Cluster.K8sConfig.Cni)); cni != "" &&
		cni != cniFlannel && cni != cniCalico {
		return fmt.Errorf("不支持的网络插件 cni: %q，可用：%s / %s",
			config.Cluster.K8sConfig.Cni, cniFlannel, cniCalico)
	}

	// 版本要在连节点之前查形状：configs/k8s 里每条 URL 都是 .../v{{.Version}}/...，
	// 写错的代价是下载 404，而那时节点已经被改过一遍了。
	if err := validateK8sVersions(config); err != nil {
		return err
	}

	// 引用的资源名必须在清单里找得到，而且这件事要在连节点之前问清楚：
	// 名字写错时若等到装的时候才报，前面几样已经装到节点上了，而报错只说"没有资源 xxx"，
	// 看不出是自己拼错了一个名字。
	if err := validateK8sResourceNames(config); err != nil {
		return err
	}

	// dockershim 在 k8s 1.24 从 kubelet 移除，而这里走的是 --cri-socket dockershim.sock
	// （见 initK8sMaster）—— 1.24 以上必然连不上 CRI。不拒绝的话，失败会发生在装完 docker、
	// 装完 kubeadm、跑到 kubeadm init 的时候：节点已经被改过一遍了。
	// version 的形状由上面的 validateK8sVersions 保证，这里必定解析得出。
	if strings.ToLower(strings.TrimSpace(config.Cluster.K8sConfig.ContainerRuntime)) == "docker" {
		major, minor, _ := parseK8sMinor(config.Cluster.K8sConfig.Version)
		if major > 1 || (major == 1 && minor >= 24) {
			return fmt.Errorf("k8s %s 不能配 containerRuntime: docker：\n"+
				"dockershim 在 1.24 已从 kubelet 移除，somcli 用的 "+
				"--cri-socket unix:///var/run/dockershim.sock 在 1.24 及以上接不上。\n"+
				"改用 containerRuntime: containerd，或把 version 降到 1.23.x；"+
				"若必须用 docker，需要节点上先装 cri-dockerd 提供 CRI 端点（somcli 尚不支持）",
				config.Cluster.K8sConfig.Version)
		}
	}

	return nil
}

// validateK8sVersions 查所有版本号的形状，在连节点之前。
//
// 两种写法过不了：
//
// 空的 k8s version —— 会拼出 `kubeadm init --kubernetes-version=`，kubeadm 报的是
// "could not parse version"；更麻烦的是这之前装的 kubeadm 二进制来自清单里的兜底版本，
// 于是"配置里没写版本"表现成"装了个没人要求的版本，然后 init 失败"。
//
// 带 v 前缀 —— configs/k8s 里每条 URL 都写着 .../v{{.Version}}/...，写 v1.30.0
// 会拼出 vv1.30.0，下载 404。这不是一个能靠猜修好的错（悄悄剥掉 v 就等于替用户改配置），
// 直接指出来更省事。
//
// 其余版本键（containerd / runc / cni-plugins / docker / cni）留空由 applyK8sDefaults
// 填默认值，所以这里只查非空值的 v 前缀。
func validateK8sVersions(config *types.ClusterConfig) error {
	k8s := config.Cluster.K8sConfig

	version := strings.TrimSpace(k8s.Version)
	if version == "" {
		return fmt.Errorf("k8sConfig.version 不能为空：请写明要装的 Kubernetes 版本（形如 1.30.0）。\n" +
			"不写的话 kubeadm init 拿不到版本号，而 kubeadm/kubelet/kubectl 已经按清单里的" +
			"兜底版本装上去了 —— 装出来的与你想要的未必是同一个版本")
	}
	if !k8sVersionPattern.MatchString(version) {
		return fmt.Errorf("k8sConfig.version 格式不对: %q，要写成完整的三段版本号（形如 1.30.0）。\n"+
			"二进制取自 https://dl.k8s.io/v<版本>/bin/...，那里只有具体的发布版本，"+
			"没有 1.30 这样的版本线；开头也不要写 v，URL 里已经有了", k8s.Version)
	}

	for _, f := range []struct {
		key   string
		value string
	}{
		{"containerdVersion", k8s.ContainerdVersion},
		{"runcVersion", k8s.RuncVersion},
		{"cniPluginsVersion", k8s.CniPluginsVersion},
		{"dockerVersion", k8s.DockerVersion},
		{"cniVersion", k8s.CniVersion},
	} {
		if v := strings.TrimSpace(f.value); strings.HasPrefix(v, "v") || strings.HasPrefix(v, "V") {
			return fmt.Errorf("k8sConfig.%s 不要带 v 前缀: %q，写成 %q。\n"+
				"清单里的下载地址是 .../v{版本}/...，带 v 会拼成 vv%s 而下载不到",
				f.key, f.value, strings.TrimLeft(v, "vV"), strings.TrimLeft(v, "vV"))
		}
	}

	return nil
}

// k8sVersionPattern 是 dl.k8s.io 上的发布版本形状：三段数字，允许 -rc.1 之类的预发布后缀。
var k8sVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?$`)

// parseK8sMinor 取版本号的主次版本：1.28.2 → (1, 28)。
// 容忍 v 前缀与次版本上的后缀（1.24-rc.0），取不出数字就 ok=false —— 宁可说"看不懂"，
// 也不要猜一个版本号出来替用户决定能不能装。
func parseK8sMinor(version string) (int, int, bool) {
	parts := strings.SplitN(strings.TrimPrefix(strings.TrimSpace(version), "v"), ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	minorField := parts[1]
	if i := strings.IndexFunc(minorField, func(r rune) bool { return r < '0' || r > '9' }); i >= 0 {
		minorField = minorField[:i]
	}
	minor, err := strconv.Atoi(minorField)
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}

// deployCNI 在 master 上部署网络插件（D5）。
//
// 之前完全没有这一步：kubeadm init 完节点是 NotReady，装完 somcli 报"创建成功"，
// 用户拿到的是一个跑不了 Pod 的集群。
//
// 走 method: manifest（一条资源，kubectl apply），而不是在这里直接拼 kubectl 命令：
// 清单的下载、分发、幂等、失败传播都是引擎已经有的能力，重写一遍就是又一处没人走的分支。
// 清单内容与 CIDR 替换写在 configs/k8s/{flannel,calico}.yaml 里。
func deployCNI(masterNode *types.RemoteNode, config *types.ClusterConfig) error {
	k8s := config.Cluster.K8sConfig
	cni := strings.ToLower(strings.TrimSpace(k8s.Cni))

	utils.PrintInfo("正在部署网络插件 %s %s（Pod 网段 %s）...", cni, k8s.CniVersion, k8s.PodNetworkCidr)

	catalog, err := loadK8sCatalog()
	if err != nil {
		return err
	}

	// 只认这两个名字：清单目录里放着别的资源也不能当 CNI 装。
	// 取值范围在 validateK8sClusterConfig 已经拦过一遍，这里是第二道，
	// 免得将来加了取值忘了同步清单时静默不装 CNI。
	if cni != cniFlannel && cni != cniCalico {
		return fmt.Errorf("不支持的网络插件: %s", k8s.Cni)
	}

	res, err := resourceFromCatalog(catalog, cni, k8s.CniVersion, []string{masterNode.IP})
	if err != nil {
		return err
	}

	if err := installer.NewInstaller().Install(res, false); err != nil {
		return fmt.Errorf("部署 %s 失败: %w", cni, err)
	}
	return nil
}

// joinMasterNodes 把第一台之外的 master 加入控制面（F9）。
//
// 原先这里是个空壳（打一行 debug 就 return nil），且失败只打警告 ——
// 配了三个 master 的用户拿到的是"创建成功"和一个单点集群，
// 而这件事要到第一台 master 宕掉、整个集群跟着不可用时才暴露。
//
// 加入控制面比加 worker 多两样东西：
//   - --control-plane：告诉 kubeadm 这台要起 apiserver/etcd，不是单纯的 kubelet；
//   - --certificate-key：取回 init 时 --upload-certs 存进 kube-system 的那份控制面证书。
//     这个 Secret 两小时后过期，所以每台 master 加入前都现场重新上传一次，
//     而不是把 init 输出里那个 key 存下来反复用。
func joinMasterNodes(config *types.ClusterConfig) error {
	masters := findMasterNodes(config)
	if len(masters) <= 1 {
		utils.PrintInfo("只有一个主节点，跳过")
		return nil
	}

	first := findFirstMasterNode(config)
	for i := range masters {
		node := masters[i]
		if node.IP == first.IP {
			continue
		}

		utils.PrintStage(fmt.Sprintf("正在加入主节点: %s", node.Host))
		startTime := time.Now()

		joinCommand, err := generateControlPlaneJoinCommand(first, &node, config)
		if err != nil {
			return err
		}

		output, err := utils.RunCommandOnNode(&node, joinCommand)
		if err != nil {
			return fmt.Errorf("主节点%s加入失败: %w\n输出: %s", node.Host, err, output)
		}
		if err := configureKubectl(&node); err != nil {
			return err
		}

		utils.PrintSuccess("✓ 主节点%s加入成功，耗时: %v", node.Host, time.Since(startTime).Round(time.Second))
	}

	return nil
}

// generateControlPlaneJoinCommand 生成一条完整的控制面加入命令。
//
// 在第一台 master 上生成，给 target 用：apiserver 要广告自己的地址，
// 不带 --apiserver-advertise-address 时 kubeadm 按默认路由挑一个网卡，
// 多网卡的机器上挑中的往往不是集群内网那张。
func generateControlPlaneJoinCommand(first, target *types.RemoteNode, config *types.ClusterConfig) (string, error) {
	joinCommand, err := generateJoinCommand(first, config)
	if err != nil {
		return "", err
	}

	certKey, err := uploadControlPlaneCerts(first)
	if err != nil {
		return "", err
	}

	joinCommand += " --control-plane --certificate-key " + certKey
	joinCommand += " --apiserver-advertise-address=" + target.IP
	return joinCommand, nil
}

// uploadControlPlaneCerts 重新把控制面证书上传到集群，返回取回它们要用的 key。
func uploadControlPlaneCerts(masterNode *types.RemoteNode) (string, error) {
	const uploadCmd = "KUBECONFIG=/etc/kubernetes/admin.conf kubeadm init phase upload-certs --upload-certs"

	output, err := utils.RunCommandOnNode(masterNode, uploadCmd)
	if err != nil {
		return "", fmt.Errorf("在主节点%s上上传控制面证书失败: %w\n输出: %s", masterNode.Host, err, output)
	}

	// kubeadm 把 key 单独打在一行里（前一行是 "Using certificate key:"）。
	// 认"64 位十六进制"而不是认提示文案：提示文案跟着 kubeadm 的语言与版本变，
	// 而 key 的形状是定的。
	for _, line := range strings.Split(output, "\n") {
		if key := strings.TrimSpace(line); isCertificateKey(key) {
			return key, nil
		}
	}
	return "", fmt.Errorf("kubeadm 没有输出可用的证书 key，输出为:\n%s", output)
}

// isCertificateKey 判断一行是不是 kubeadm 的控制面证书 key。
func isCertificateKey(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

// joinWorkerNodes 加入工作节点。
//
// join 命令在 master 上现场生成（kubeadm token create --print-join-command），
// 而不是从 kubeadm init 的输出里刮出来再存文件：
//   - init 输出里的那条命令是跨两行的，按行提取必然丢 --discovery-token-ca-cert-hash（D4）；
//   - 存文件还带来第二个问题 —— 默认 token 24 小时过期，扩容时读到的是一条过期命令。
//
// --print-join-command 打的是完整一行，且每次都是新 token。
func joinWorkerNodes(config *types.ClusterConfig) error {
	workers := make([]types.RemoteNode, 0, len(config.Cluster.Nodes))
	for i := range config.Cluster.Nodes {
		if strings.EqualFold(config.Cluster.Nodes[i].Role, "worker") {
			workers = append(workers, config.Cluster.Nodes[i])
		}
	}
	if len(workers) == 0 {
		utils.PrintInfo("配置中没有 worker 节点，跳过")
		return nil
	}

	masterNode := findFirstMasterNode(config)
	if masterNode == nil {
		return fmt.Errorf("配置中没有找到主节点，无法生成加入命令")
	}

	joinCommand, err := generateJoinCommand(masterNode, config)
	if err != nil {
		utils.PrintError("生成加入命令失败: %v", err)
		return err
	}

	for i := range workers {
		node := workers[i]
		utils.PrintStage(fmt.Sprintf("正在加入工作节点: %s", node.Host))
		startTime := time.Now()

		output, err := utils.RunCommandOnNode(&node, joinCommand)
		if err != nil {
			utils.PrintError("工作节点加入失败: %v", err)
			return fmt.Errorf("工作节点%s加入失败: %w\n输出: %s", node.Host, err, output)
		}

		duration := time.Since(startTime)
		utils.PrintSuccess("✓ 节点%s加入成功，耗时: %v", node.Host, duration.Round(time.Second))
	}

	return nil
}

// generateJoinCommand 在 master 上生成一条完整的 worker join 命令。
func generateJoinCommand(masterNode *types.RemoteNode, config *types.ClusterConfig) (string, error) {
	const printJoin = "KUBECONFIG=/etc/kubernetes/admin.conf kubeadm token create --print-join-command"

	output, err := utils.RunCommandOnNode(masterNode, printJoin)
	if err != nil {
		return "", fmt.Errorf("在主节点%s上生成加入命令失败: %w\n输出: %s", masterNode.Host, err, output)
	}

	// 输出里可能混着 kubeadm 自己的提示行，只认以 kubeadm join 开头的那一行。
	joinCommand := ""
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "kubeadm join") {
			joinCommand = line
			break
		}
	}
	if joinCommand == "" {
		return "", fmt.Errorf("kubeadm 没有输出可用的加入命令，输出为:\n%s", output)
	}

	// join 也要指明 CRI 端点：节点上同时装了 docker 与 containerd 时，
	// kubeadm 检测到多个 socket 会直接报错要求显式指定。
	if !strings.Contains(joinCommand, "--cri-socket") {
		joinCommand += " --cri-socket " + criSocket(config.Cluster.K8sConfig.ContainerRuntime)
	}

	utils.PrintInfo("加入命令: %s", joinCommand)
	return joinCommand, nil
}

// criSocket 按容器运行时给出 kubeadm 要用的 CRI 端点。
//
// docker 走的 dockershim 在 1.24 已被移除，这条分支只对 1.23 及以下成立 ——
// 版本与运行时不匹配的组合在 validateK8sClusterConfig 里已经被拒掉了。
func criSocket(runtime string) string {
	if strings.EqualFold(strings.TrimSpace(runtime), "docker") {
		return "unix:///var/run/dockershim.sock"
	}
	return "unix:///var/run/containerd/containerd.sock"
}

// printK8sClusterInfo 打印 Kubernetes 集群信息
func printK8sClusterInfo(config *types.ClusterConfig, masterNode *types.RemoteNode) error {
	utils.PrintInfo("正在获取集群节点信息...")
	output, err := utils.RunCommandOnNode(masterNode, "kubectl get nodes")
	if err != nil {
		return fmt.Errorf("获取集群节点失败: %w", err)
	}

	utils.PrintSuccess("\n✓ Kubernetes集群创建成功!")
	utils.PrintInfo("\n=== 集群信息 ===")
	utils.PrintInfo("集群名称: %s", config.Cluster.Name)
	utils.PrintInfo("主节点: %s (%s)", masterNode.Host, masterNode.IP)
	utils.PrintInfo("\n集群节点:")
	fmt.Println(output)

	utils.PrintInfo("\n=== 验证指南 ===")
	utils.PrintInfo("1. 检查集群状态:")
	utils.PrintInfo("   kubectl get nodes")
	utils.PrintInfo("   kubectl get pods --all-namespaces")

	utils.PrintInfo("\n2. 部署测试应用:")
	utils.PrintInfo("   kubectl create deployment nginx --image=nginx")
	utils.PrintInfo("   kubectl expose deployment nginx --port=80 --type=NodePort")

	infoFile := filepath.Join(utils.GetWorkDir(), "k8s-cluster-info.txt")
	infoContent := fmt.Sprintf("集群名称: %s\n主节点: %s\n\n节点:\n%s",
		config.Cluster.Name, masterNode.Host, output)
	if err := utils.WriteStringToFile(infoFile, infoContent); err != nil {
		utils.PrintWarning("保存集群信息失败: %v", err)
	} else {
		utils.PrintInfo("\n集群信息已保存到: %s", infoFile)
	}

	return nil
}

// findMasterNodes 查找所有主节点，返回主节点列表
func findMasterNodes(config *types.ClusterConfig) []types.RemoteNode {
	masterList := make([]types.RemoteNode, 0, len(config.Cluster.Nodes)/2)
	for i := range config.Cluster.Nodes {
		if strings.ToLower(config.Cluster.Nodes[i].Role) == "master" {
			masterList = append(masterList, config.Cluster.Nodes[i])
		}
	}
	return masterList
}

// findFirstMasterNode 查找第一个主节点，如果没有找到则返回nil
func findFirstMasterNode(config *types.ClusterConfig) *types.RemoteNode {
	masterList := findMasterNodes(config)
	if len(masterList) > 0 {
		return &masterList[0]
	}
	return nil
}

// 移除 Kubernetes 集群
func RemoveK8sCluster(config *types.ClusterConfig, force bool) error {
	startTime := time.Now()
	utils.PrintBanner(fmt.Sprintf("正在移除Kubernetes集群: %s", config.Cluster.Name))
	utils.PrintInfo("开始时间: %s", startTime.Format("2006-01-02 15:04:05"))

	// 补默认值：reset 要带 --cri-socket，而端点取的是 containerRuntime。
	// 不补的话配置里没写 runtime 时会按 containerd 拼端点，docker 集群的 reset
	// 就会因为检测到多个 socket 又没显式指定而失败。
	applyK8sDefaults(config)

	if !force {
		if !utils.AskForConfirmation("确定要移除Kubernetes集群吗？") {
			utils.PrintWarning("操作已取消")
			return fmt.Errorf("操作已取消")
		}
	}

	utils.PrintStage("== 集群移除流程 ==")
	for _, node := range resetOrder(config) {
		utils.PrintStage(fmt.Sprintf("正在重置节点: %s (%s)", node.Host, node.IP))
		startTime := time.Now()

		if err := resetK8sNode(&node, config); err != nil {
			utils.PrintError("节点重置失败: %v", err)
			return err
		}

		duration := time.Since(startTime)
		utils.PrintSuccess("✓ 节点%s重置完成，耗时: %v", node.Host, duration.Round(time.Second))
	}

	duration := time.Since(startTime)
	utils.PrintSuccess("\n✓ Kubernetes集群 '%s' 移除成功!", config.Cluster.Name)
	utils.PrintInfo("总执行时间: %v", duration.Round(time.Second))

	return nil
}

// resetOrder 给出重置节点的顺序：worker 先，第一台 master 最后。
//
// 顺序是有讲究的：`kubeadm reset` 会顺手把自己从集群里摘掉（删 Node 对象、
// 从 etcd 成员列表里退出），而这要 apiserver 还活着。反过来先拆第一台 master 的话，
// 后面每台都少做这一步，只是"本机文件删了"，别处的集群视图里它们还在。
func resetOrder(config *types.ClusterConfig) []types.RemoteNode {
	first := findFirstMasterNode(config)

	ordered := make([]types.RemoteNode, 0, len(config.Cluster.Nodes))
	for i := range config.Cluster.Nodes {
		if !strings.EqualFold(config.Cluster.Nodes[i].Role, "master") {
			ordered = append(ordered, config.Cluster.Nodes[i])
		}
	}
	for i := range config.Cluster.Nodes {
		n := config.Cluster.Nodes[i]
		if strings.EqualFold(n.Role, "master") && (first == nil || n.IP != first.IP) {
			ordered = append(ordered, n)
		}
	}
	if first != nil {
		ordered = append(ordered, *first)
	}
	return ordered
}

// resetK8sNode 把一个节点上的 k8s 抹掉。
//
// 「抹干净」的判据是这台机器还能再装一遍（SC-K11）。因此除了 kubeadm reset，
// 还得处理它明确不管的两样东西：
//   - CNI 留下的网桥与隧道口。不删的话下一次装出来的 Pod 会拿到上一次网段里的地址，
//     表现是跨节点不通，而不是"上次没清干净"；
//   - `/etc/cni/net.d`。kubeadm reset 只在有 CNI 配置时提示你自己删。
func resetK8sNode(node *types.RemoteNode, config *types.ClusterConfig) error {
	utils.PrintInfo("正在执行kubeadm reset...")
	// 必须指明 CRI 端点：节点上同时装了 docker 与 containerd 时，
	// kubeadm 检测到多个 socket 直接报错要求显式指定 —— 与 join 是同一个道理。
	resetCmd := "kubeadm reset -f --cri-socket " + criSocket(config.Cluster.K8sConfig.ContainerRuntime)
	if output, err := utils.RunCommandOnNode(node, resetCmd); err != nil {
		return fmt.Errorf("节点%s重置失败: %w\n输出: %s", node.Host, err, output)
	}

	utils.PrintInfo("正在清理配置...")
	cleanupCmds := []string{
		"rm -rf /etc/cni/net.d",
		"rm -rf $HOME/.kube",
		"rm -rf /etc/kubernetes",
		// 每种 CNI 留下的口不一样，删不掉的（本来就没有）忽略。
		"ip link delete cni0 2>/dev/null || true",
		"ip link delete flannel.1 2>/dev/null || true",
		"ip link delete tunl0 2>/dev/null || true",
		"ip link delete vxlan.calico 2>/dev/null || true",
		"ip link delete kube-ipvs0 2>/dev/null || true",
	}
	for _, cmd := range cleanupCmds {
		if output, err := utils.RunCommandOnNode(node, cmd); err != nil {
			// 清理失败不致命，但要说出是哪一条没成 —— 下一次装不上时这行日志是唯一线索。
			utils.PrintWarning("清理操作失败: %s: %v\n输出: %s", cmd, err, output)
		}
	}

	// kube-proxy 写的 iptables 规则由 kubeadm reset 提示"需要自己清"，这里不动：
	// 那些链与宿主上别的规则混在一条表里，无条件 flush 会顺手废掉用户自己的规则
	// （最典型的是 docker 的 NAT）。重装不受它影响 —— kube-proxy 起来会重建。
	utils.PrintInfo("提示：kube-proxy 留下的 iptables 规则未清理，重装时会被覆盖；" +
		"要彻底清可在节点上执行 iptables -t nat -F（会影响该机上其他规则）")

	return nil
}
