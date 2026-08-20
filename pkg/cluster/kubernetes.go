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
	"strconv"
	"strings"
	"time"

	"github.com/structure-projects/somcli/pkg/installer"
	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

const (
	containerdServiceTemplate = `[Unit]
Description=containerd container runtime
Documentation=https://containerd.io
After=network.target local-fs.target

[Service]
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/local/bin/containerd
Restart=always
RestartSec=5
Delegate=yes
KillMode=process
OOMScoreAdjust=-999
LimitNOFILE=1048576
LimitNPROC=infinity
LimitCORE=infinity

[Install]
WantedBy=multi-user.target`

	// basePackagesCmd 在节点上装 kubeadm preflight 要求的那几个包。
	//
	// 包管理器的判断写在**生成的 shell 里**而不是 Go 侧：装包发生在目标节点上，
	// 拿操作机的发行版去挑包管理器，一到远程就是错的（与 --sudo 同一个道理）。
	// 原先这里写死 yum install -y ...，在 debian/ubuntu 节点上第一步就失败，
	// 集群安装根本走不到后面。
	//
	// 这只是让 k8s 安装能在非 yum 发行版上跑起来的最小处理，完整的发行版抽象仍属 M3。
	basePackagesCmd = `if command -v apt-get >/dev/null 2>&1; then
  DEBIAN_FRONTEND=noninteractive apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y socat conntrack ebtables ipset
elif command -v dnf >/dev/null 2>&1; then
  dnf install -y --allowerasing socat conntrack-tools ebtables ipset
elif command -v yum >/dev/null 2>&1; then
  yum install -y socat conntrack ebtables ipset
elif command -v apk >/dev/null 2>&1; then
  apk add --no-cache socat conntrack-tools ebtables ipset
elif command -v zypper >/dev/null 2>&1; then
  zypper --non-interactive install socat conntrack-tools ebtables ipset
else
  echo "认不出节点上的包管理器，请先手工安装 socat conntrack ebtables ipset" >&2
  exit 1
fi`
)

// 组件的默认版本。
//
// 必须有默认值：配置里不写 containerdVersion 时，URL 会拼成
// .../download/v/containerd--linux-amd64.tar.gz —— 下载 404，而报错只说"下载失败"，
// 用户根本看不出是自己少写了一个键。
// 这些值都对着 k8s 1.28 那一代；M2.3 把安装内容外置到 configs/k8s/*.yaml 之后，
// 版本会跟着搬到配置里，这里只留兜底。
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
	if err := installDependencies(config); err != nil {
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

	// 5. 工作节点加入
	utils.PrintStage("== 工作节点加入 ==")
	if err := joinWorkerNodes(config); err != nil {
		utils.PrintError("工作节点加入失败: %v", err)
		return fmt.Errorf("工作节点加入失败: %w", err)
	}
	utils.PrintSuccess("✓ 工作节点加入完成")

	// 6. 集群信息展示
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

// installDependencies
func installDependencies(config *types.ClusterConfig) error {
	utils.PrintInfo("正在准备安装Kubernetes %s...", config.Cluster.K8sConfig.Version)

	// 获取所有节点IP
	hosts := getAllNodesIP(config)

	// 1. 安装基础依赖
	if err := installBaseDependencies(config, hosts); err != nil {
		return err
	}

	// 2. 安装容器运行时
	runtime := config.Cluster.K8sConfig.ContainerRuntime
	if runtime == "" {
		runtime = "containerd" // 默认使用containerd
	}

	switch runtime {
	case "docker":
		if err := installDocker(config, hosts); err != nil {
			return err
		}

	case "containerd":
		if err := installContainerd(config, hosts); err != nil {
			return err
		}
	default:
		return fmt.Errorf("不支持的容器运行时: %s", runtime)
	}

	// 3. 安装Kubernetes组件
	return installK8sComponents(config, hosts)
}

// installDocker 安装Docker
func installDocker(config *types.ClusterConfig, hosts []string) error {
	utils.PrintInfo("正在安装Docker...")

	dockerVersion := config.Cluster.K8sConfig.DockerVersion

	// 定义Docker资源
	//
	// 本文件里所有资源的 method 都是 script：解压、install、chmod、systemctl 全写在
	// post_install 里，method 字段过去零消费，写 "binary"/"package" 只是自我描述。
	// 分发实现之后再挂着那两个值就会与脚本撞车（binary 再装一遍、package 去装一个叫
	// base-dependencies 的包）。这是 method 分发落地时的最小适配，
	// 真正改造成 method: binary / package 由 M2 集群编排一起做。
	dockerResource := types.Resource{
		Name:    "docker",
		Version: dockerVersion,
		Method:  "script",
		URLs: []string{
			"https://download.docker.com/linux/static/stable/x86_64/docker-{{.Version}}.tgz",
		},
		PostInstall: []string{
			" tar xzvf {{.CacheDir}}/{{.Name}}-{{.Version}}.tgz -C /usr/local/bin",
			" chmod +x /usr/local/bin/docker*",
			" groupadd docker || true",
			" usermod -aG docker $USER",
			" mkdir -p /etc/docker",
			" systemctl enable docker",
			" systemctl start docker",
		},
		ExtraFiles: map[string]string{
			"/etc/docker/daemon.json": `{
                "exec-opts": ["native.cgroupdriver=systemd"],
                "log-driver": "json-file",
                "log-opts": {"max-size": "100m"},
                "storage-driver": "overlay2"
            }`,
		},
		Hosts:  hosts,
		Target: "{{.Name}}-{{.Version}}.tgz",
	}

	installer := installer.NewInstaller()

	return installer.Install(dockerResource, true)
}

// installContainerd 安装Containerd
func installContainerd(config *types.ClusterConfig, hosts []string) error {
	utils.PrintInfo("正在安装Containerd...")

	installer := installer.NewInstaller()

	// 定义 CNI 插件资源（/opt/cni/bin 下那批二进制，不是网络方案本身）。
	//
	// 名字原先也叫 "containerd"，和下面真正的 containerd 资源同名 —— 幂等状态是按
	// 名字+版本记的，两条资源共用一个名字，先装的那条会被后装的覆盖掉，
	// 于是"这台机器装过 cni-plugins 没有"永远查不到正确答案。
	cniResource := types.Resource{
		Name:    "cni-plugins",
		Version: config.Cluster.K8sConfig.CniPluginsVersion,
		Method:  "script",
		URLs: []string{
			"https://github.com/containernetworking/plugins/releases/download/v{{.Version}}/cni-plugins-linux-amd64-v{{.Version}}.tgz",
		},
		PostInstall: []string{
			" mkdir -p /opt/cni/bin",
			" tar Cxzvf /opt/cni/bin {{.CacheDir}}/cni-plugins-linux-amd64-v{{.Version}}.tgz",
		},
		Hosts:  hosts,
		Target: "{{.Filename}}",
	}
	if err := installer.Install(cniResource, false); err != nil {
		return fmt.Errorf("安装 CNI 插件失败: %w", err)
	}

	// 定义Containerd资源
	runcResource := types.Resource{
		Name:    "runc",
		Version: config.Cluster.K8sConfig.RuncVersion,
		Method:  "script",
		URLs: []string{
			"https://github.com/opencontainers/runc/releases/download/v{{.Version}}/runc.amd64",
		},
		PostInstall: []string{
			" install -m 755 {{.CacheDir}}/runc.amd64 /usr/local/sbin/runc",
		},
		Hosts:  hosts,
		Target: "{{.Filename}}",
	}

	if err := installer.Install(runcResource, false); err != nil {
		return fmt.Errorf("安装 runc 失败: %w", err)
	}

	// 定义Containerd资源
	//
	// post_install 分步拼而不是写成一整块字面量：镜像仓库相关的两条 sed 只有在
	// imageRepository 真的配了的时候才能加。此前是无条件拼进去的，配置里不写仓库时
	// 生成的是 sed 's|k8s.gcr.io||g' 和 sandbox_image = "/pause:" ——
	// 配置被改坏，containerd 拉不到 pause 镜像，而报错指向的是"沙箱镜像拉取失败"。
	containerdCmds := []string{
		"tar Cxzvf /usr/local {{.CacheDir}}/containerd-{{.Version}}-linux-amd64.tar.gz",
		"mkdir -p /etc/containerd",
		"containerd config default |  tee /etc/containerd/config.toml >/dev/null",
	}
	if repo := strings.TrimSpace(config.Cluster.K8sConfig.ImageRepository); repo != "" {
		containerdCmds = append(containerdCmds,
			fmt.Sprintf("sed -i 's|k8s.gcr.io|%s|g' /etc/containerd/config.toml", repo))
		if pause := strings.TrimSpace(config.Cluster.K8sConfig.PauseImageVersion); pause != "" {
			containerdCmds = append(containerdCmds,
				fmt.Sprintf(" sed -i 's|sandbox_image = \".*\"|sandbox_image = \"%s/pause:%s\"|g' /etc/containerd/config.toml",
					repo, pause))
		}
	}
	containerdCmds = append(containerdCmds,
		// F7：kubeadm 默认用 systemd 驱动，containerd 默认配置里是 cgroupfs。
		// 两边不一致的表现是 kubelet 反复重启，而报错完全指不到根因上。
		"sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml",
		// 改完必须验：containerd 换过配置格式（v2/v3 的键位置不同），
		// sed 没匹配上时它照样退 0，那就成了"改过了"的假象。
		"grep -q 'SystemdCgroup = true' /etc/containerd/config.toml",
		"systemctl daemon-reload",
		"systemctl enable --now containerd",
	)

	containerdResource := types.Resource{
		Name:    "containerd",
		Version: config.Cluster.K8sConfig.ContainerdVersion,
		Method:  "script",
		URLs: []string{
			"https://github.com/containerd/containerd/releases/download/v{{.Version}}/containerd-{{.Version}}-linux-amd64.tar.gz",
		},
		PostInstall: containerdCmds,
		ExtraFiles: map[string]string{
			"/etc/systemd/system/containerd.service": containerdServiceTemplate,
		},
		Hosts:  hosts,
		Target: "{{.Filename}}",
	}

	return installer.Install(containerdResource, false)
}

// installK8sComponents 安装Kubernetes组件
func installK8sComponents(config *types.ClusterConfig, hosts []string) error {
	utils.PrintInfo("正在安装Kubernetes组件...")

	k8sVersion := config.Cluster.K8sConfig.Version

	// 定义Kubernetes组件资源
	k8sResource := types.Resource{
		Name:    "kubernetes",
		Version: k8sVersion,
		Method:  "script",
		URLs: []string{
			"https://dl.k8s.io/v{{.Version}}/bin/linux/amd64/kubeadm",
			"https://dl.k8s.io/v{{.Version}}/bin/linux/amd64/kubelet",
			"https://dl.k8s.io/v{{.Version}}/bin/linux/amd64/kubectl",
			"https://structured.oss-cn-beijing.aliyuncs.com/somwork/service/kubelet.service",
		},
		PostInstall: []string{
			" install -o root -g root -m 0755 {{.CacheDir}}/kubeadm /usr/local/bin/kubeadm",
			" install -o root -g root -m 0755 {{.CacheDir}}/kubelet /usr/local/bin/kubelet",
			" install -o root -g root -m 0755 {{.CacheDir}}/kubectl /usr/local/bin/kubectl",
			" mkdir -p /etc/systemd/system/kubelet.service.d",
			" install -o root -g root -m 0644 {{.CacheDir}}/kubelet.service /etc/systemd/system/kubelet.service",
			" systemctl daemon-reload",
			" systemctl enable --now kubelet",
		},
		Hosts:  hosts,
		Target: "{{.Filename}}",
	}

	installer := installer.NewInstaller()
	return installer.Install(k8sResource, false)
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
	// 只写在 init 上还不够让多 master 真的成立（joinMaster 仍是空壳），但少了这一句，
	// 后来补上的 join 也接不到正确的地址。
	if endpoint := strings.TrimSpace(config.Cluster.K8sConfig.ControlPlaneEndpoint); endpoint != "" {
		initCmd += fmt.Sprintf(" --control-plane-endpoint=%s", endpoint)
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
	cmds := []string{
		"mkdir -p $HOME/.kube",
		" cp -i /etc/kubernetes/admin.conf $HOME/.kube/config",
		" chown $(id -u):$(id -g) $HOME/.kube/config",
	}

	for _, cmd := range cmds {
		if _, err := utils.RunCommandOnNode(node, cmd); err != nil {
			utils.PrintError("命令执行失败: %s: %v", cmd, err)
			return fmt.Errorf("kubectl配置失败: %w", err)
		}
	}

	// 检查是否其他主节点，同步主节点配置
	masterList := findMasterNodes(config)
	for _, masterNode := range masterList {
		if masterNode.IP != findFirstMasterNode(config).IP {
			err := joinMaster(masterNode, config)
			if nil != err {
				utils.PrintWarning("master %s join failed!", masterNode.IP)
			}
		}
	}

	duration := time.Since(startTime)
	utils.PrintSuccess("✓ 主节点初始化完成，耗时: %v", duration.Round(time.Second))
	return nil
}

// generateKubeadmConfig 生成kubeadm配置文件
func generateKubeadmConfig(node *types.RemoteNode, config *types.ClusterConfig) (string, error) {
	runtime := config.Cluster.K8sConfig.ContainerRuntime
	if runtime == "" {
		runtime = "containerd"
	}

	criSocket := "unix:///var/run/containerd/containerd.sock"
	if runtime == "docker" {
		criSocket = "unix:///var/run/dockershim.sock"
	}

	kubeadmConfig := fmt.Sprintf(`apiVersion: kubeadm.k8s.io/v1beta3
kind: InitConfiguration
nodeRegistration:
  criSocket: %s
  name: %s
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
kubernetesVersion: %s
apiServer:
  certSANs:
  - "%s"
controlPlaneEndpoint: "%s:6443"
networking:
  podSubnet: "%s"
  serviceSubnet: "%s"
`, criSocket, node.Host,
		config.Cluster.K8sConfig.Version,
		node.IP, node.IP,
		config.Cluster.K8sConfig.PodNetworkCidr,
		config.Cluster.K8sConfig.ServiceCidr)

	// 添加镜像仓库配置
	if config.Cluster.K8sConfig.ImageRepository != "" {
		kubeadmConfig += fmt.Sprintf("imageRepository: %s\n", config.Cluster.K8sConfig.ImageRepository)
	}

	return kubeadmConfig, nil
}

// getAllNodesIP 获取所有节点IP
func getAllNodesIP(config *types.ClusterConfig) []string {
	hosts := []string{}
	for _, node := range config.Cluster.Nodes {
		hosts = append(hosts, node.IP)
	}
	return hosts
}

// installBaseDependencies 安装基础依赖，并把 kubeadm preflight 要求的内核前置落到节点上。
func installBaseDependencies(config *types.ClusterConfig, hosts []string) error {
	utils.PrintInfo("正在安装基础依赖...")

	commands := []string{
		basePackagesCmd,
		" swapoff -a",
		" sed -i '/ swap / s/^/#/' /etc/fstab",
		" modprobe overlay",
		" modprobe br_netfilter",
		" sysctl --system",
		// F8：写了文件、跑了 sysctl --system，不等于参数真的生效（模块没加载时这两个
		// 键根本不存在，sysctl --system 只会打一行警告然后退 0）。当场验一遍，
		// 让它在这里失败，而不是等 kubeadm preflight 报一个指不到根因的错。
		` test "$(sysctl -n net.bridge.bridge-nf-call-iptables)" = 1`,
		` test "$(sysctl -n net.ipv4.ip_forward)" = 1`,
	}

	baseDeps := types.Resource{
		Name:    "base-dependencies",
		Version: "",
		Method:  "script",
		// F8：这两个文件此前从来没人写，于是上面的 sysctl --system 无事可做，
		// 而 modprobe 的效果重启就没了。
		ExtraFiles: map[string]string{
			"/etc/modules-load.d/k8s.conf": "overlay\nbr_netfilter\n",
			"/etc/sysctl.d/k8s.conf": "net.bridge.bridge-nf-call-iptables  = 1\n" +
				"net.bridge.bridge-nf-call-ip6tables = 1\n" +
				"net.ipv4.ip_forward                 = 1\n",
		},
		PostInstall: commands,
		Hosts:       hosts,
	}

	installer := installer.NewInstaller()
	return installer.Install(baseDeps, true)
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
	if err := prepareK8sNodes(config); err != nil {
		utils.PrintError("节点准备失败: %v", err)
		return fmt.Errorf("节点准备失败: %w", err)
	}

	return nil
}

// prepareK8sNodes 准备所有Kubernetes节点
func prepareK8sNodes(config *types.ClusterConfig) error {
	var hostsEntries strings.Builder
	for _, node := range config.Cluster.Nodes {
		hostsEntries.WriteString(fmt.Sprintf("%s\t%s\n", node.IP, node.Host))
	}

	for _, node := range config.Cluster.Nodes {
		utils.PrintStage(fmt.Sprintf("准备节点: %s (%s)", node.Host, node.IP))
		startTime := time.Now()

		utils.PrintInfo("正在检查操作系统...")
		if err := checkAndConfigureOS(&node); err != nil {
			utils.PrintError("操作系统配置失败: %v", err)
			return fmt.Errorf("节点%s操作系统配置失败: %w", node.Host, err)
		}

		utils.PrintInfo("正在配置hosts文件...")
		if err := configureHostsFile(&node, hostsEntries.String()); err != nil {
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
	if !strings.Contains(strings.ToLower(arch), "x86_64") && !strings.Contains(strings.ToLower(arch), "amd64") {
		return fmt.Errorf("不支持的CPU架构: %s，仅支持x86_64/amd64", arch)
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

	// dockershim 在 k8s 1.24 从 kubelet 移除，而这里走的是 --cri-socket dockershim.sock
	// （见 initK8sMaster）—— 1.24 以上必然连不上 CRI。不拒绝的话，失败会发生在装完 docker、
	// 装完 kubeadm、跑到 kubeadm init 的时候：节点已经被改过一遍了。
	if strings.ToLower(strings.TrimSpace(config.Cluster.K8sConfig.ContainerRuntime)) == "docker" {
		major, minor, ok := parseK8sMinor(config.Cluster.K8sConfig.Version)
		if !ok {
			return fmt.Errorf("containerRuntime: docker 时必须写明 version（形如 1.23.17），"+
				"当前为 %q：dockershim 在 1.24 已被移除，"+
				"不知道版本就无法判断这套配置能不能装成",
				config.Cluster.K8sConfig.Version)
		}
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
// 用户拿到的是一个跑不了 Pod 的集群。calico / flannel 在整个 Go 代码里零命中。
//
// 走 method: manifest（一条资源，kubectl apply），而不是在这里直接拼 kubectl 命令：
// 清单的下载、分发、幂等、失败传播都是引擎已经有的能力，重写一遍就是又一处没人走的分支。
func deployCNI(masterNode *types.RemoteNode, config *types.ClusterConfig) error {
	k8s := config.Cluster.K8sConfig
	cni := strings.ToLower(strings.TrimSpace(k8s.Cni))
	podCidr := strings.TrimSpace(k8s.PodNetworkCidr)

	utils.PrintInfo("正在部署网络插件 %s %s（Pod 网段 %s）...", cni, k8s.CniVersion, podCidr)

	var res types.Resource
	switch cni {
	case cniFlannel:
		res = types.Resource{
			Name:    "flannel",
			Version: k8s.CniVersion,
			Method:  "manifest",
			URLs: []string{
				"https://github.com/flannel-io/flannel/releases/download/v{{.Version}}/kube-flannel.yml",
			},
			Target: "{{.Filename}}",
			PreInstall: []string{
				// 清单里写死 10.244.0.0/16。与 kubeadm --pod-network-cidr 不一致时，
				// Pod 拿到的地址不在集群网段里，表现是跨节点不通而不是"配错了"。
				fmt.Sprintf("sed -i 's|10.244.0.0/16|%s|g' {{.CacheDir}}/kube-flannel.yml", podCidr),
				// 改完必须验：清单换过键名/格式时 sed 匹配不上也退 0，
				// 那就成了"改过了"的假象，而错要到跨节点通信时才暴露。
				fmt.Sprintf("grep -q '%s' {{.CacheDir}}/kube-flannel.yml", podCidr),
			},
			Hosts: []string{masterNode.IP},
		}
	case cniCalico:
		res = types.Resource{
			Name:    "calico",
			Version: k8s.CniVersion,
			Method:  "manifest",
			URLs: []string{
				"https://raw.githubusercontent.com/projectcalico/calico/v{{.Version}}/manifests/calico.yaml",
			},
			Target: "{{.Filename}}",
			PreInstall: []string{
				// calico 清单里 CALICO_IPV4POOL_CIDR 是注释掉的，不显式设的话它用自己的
				// 默认 192.168.0.0/16 —— 与 kubeadm 的 --pod-network-cidr 对不上。
				// 去掉注释后缩进正好与同级列表项对齐（官方文档就是这么改的）。
				"sed -i 's|# - name: CALICO_IPV4POOL_CIDR|- name: CALICO_IPV4POOL_CIDR|' {{.CacheDir}}/calico.yaml",
				fmt.Sprintf("sed -i 's|#   value: \"192.168.0.0/16\"|  value: \"%s\"|' {{.CacheDir}}/calico.yaml", podCidr),
				"grep -q '^ *- name: CALICO_IPV4POOL_CIDR' {{.CacheDir}}/calico.yaml",
				fmt.Sprintf("grep -q '^ *value: \"%s\"' {{.CacheDir}}/calico.yaml", podCidr),
			},
			Hosts: []string{masterNode.IP},
		}
	default:
		// 走不到：validateK8sClusterConfig 已经拒过了。留着是为了万一将来加了取值
		// 忘了同步这里时，报错指向的是"没实现"而不是静默不装 CNI。
		return fmt.Errorf("不支持的网络插件: %s", k8s.Cni)
	}

	if err := installer.NewInstaller().Install(res, false); err != nil {
		return fmt.Errorf("部署 %s 失败: %w", cni, err)
	}
	return nil
}

// 其他的master 上执行加入master
func joinMaster(node types.RemoteNode, config *types.ClusterConfig) error {
	//加入master节点
	utils.PrintDebug("node %v, config, %v", node, config)
	return nil
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

		output, err := utils.RunCommandOnNode(&node, " "+joinCommand)
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

	if !force {
		if !utils.AskForConfirmation("确定要移除Kubernetes集群吗？") {
			utils.PrintWarning("操作已取消")
			return fmt.Errorf("操作已取消")
		}
	}

	utils.PrintStage("== 集群移除流程 ==")
	for _, node := range config.Cluster.Nodes {
		utils.PrintStage(fmt.Sprintf("正在重置节点: %s (%s)", node.Host, node.IP))
		startTime := time.Now()

		utils.PrintInfo("正在执行kubeadm reset...")
		if _, err := utils.RunCommandOnNode(&node, " kubeadm reset -f"); err != nil {
			utils.PrintError("节点重置失败: %v", err)
			return fmt.Errorf("节点%s重置失败: %w", node.Host, err)
		}

		utils.PrintInfo("正在清理配置...")
		cleanupCmds := []string{
			" rm -rf /etc/cni/net.d",
			" rm -rf $HOME/.kube",
			" rm -rf /etc/kubernetes",
		}

		for _, cmd := range cleanupCmds {
			if _, err := utils.RunCommandOnNode(&node, cmd); err != nil {
				utils.PrintWarning("清理操作失败: %s: %v", cmd, err)
			}
		}

		duration := time.Since(startTime)
		utils.PrintSuccess("✓ 节点%s重置完成，耗时: %v", node.Host, duration.Round(time.Second))
	}

	duration := time.Since(startTime)
	utils.PrintSuccess("\n✓ Kubernetes集群 '%s' 移除成功!", config.Cluster.Name)
	utils.PrintInfo("总执行时间: %v", duration.Round(time.Second))

	return nil
}
