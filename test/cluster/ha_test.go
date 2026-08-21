//go:build cluster

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
	"testing"
	"time"
)

// TestSC_K04_MultiMaster 配了两个 master 就必须装出两个控制面（F9）。
//
// 修之前 `joinMaster` 是个空壳（打一行 debug 就 return nil），而且失败只打警告：
// 配三个 master 的用户拿到的是"创建成功"和一个单点集群 —— 这件事要到第一台 master
// 宕掉、整个集群跟着不可用时才暴露。
//
// 判据一路收紧，因为每一层都有各自的假绿：
//   - 退出 0：空壳实现也成立，毫无意义；
//   - 两个节点出现在 kubectl get nodes：worker 加进来也是两个，所以要看 control-plane 角色；
//   - 第二台带 control-plane 角色：光有角色标签不代表它真的在跑控制面；
//   - 第二台上有 etcd 与 apiserver 的静态 Pod：这才是"控制面真的多了一份"。
//
// 另外验 kubeconfig 里的 server 是那个稳定入口而不是某台机器的地址：指着 master1 的话，
// master1 一停，另一台 master 还活着但所有客户端都连不上，等于白装。
func TestSC_K04_MultiMaster(t *testing.T) {
	t.Cleanup(func() {
		if t.Failed() {
			dumpNodeLogs()
		}
	})

	resetCluster(t, master, master2, worker)

	cfg := clusterConfigWithEndpoint(t, "sc-k04", "1.28.2", "containerd", "", lbEndpoint,
		master, master2)
	code, out := runSomcli(t, "cluster", "create", "-f", cfg)
	if code != 0 {
		t.Fatalf("两 master 的 cluster create 退出码 = %d\n输出：\n%s", code, out)
	}

	waitFor(t, "两个节点都带 control-plane 角色", 8*time.Minute, func() (bool, string) {
		got, err := kubectl(t, "get", "nodes", "--no-headers")
		if err != nil {
			return false, err.Error()
		}
		controlPlanes := 0
		for _, line := range strings.Split(strings.TrimSpace(got), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 3 && strings.Contains(fields[2], "control-plane") {
				controlPlanes++
			}
		}
		return controlPlanes == 2, got
	})

	// 静态 Pod 的名字是 <组件>-<节点名>，因此这两条同时断言了"在第二台上"。
	for _, pod := range []string{"etcd-" + master2.host, "kube-apiserver-" + master2.host} {
		waitFor(t, pod+" 处于 Running", 5*time.Minute, func() (bool, string) {
			got, err := kubectl(t, "get", "pod", pod, "-n", "kube-system",
				"-o", "jsonpath={.status.phase}")
			if err != nil {
				return false, err.Error()
			}
			return strings.TrimSpace(got) == "Running", got
		})
	}

	// etcd 成员数要真的是 2：apiserver 起来了但 etcd 只是"加进去又被踢掉"的话，
	// 上面那条断言在窗口期内也可能成立。
	members, err := ssh(master, "kubectl -n kube-system exec etcd-"+master.host+
		" -- etcdctl --endpoints=https://127.0.0.1:2379"+
		" --cacert=/etc/kubernetes/pki/etcd/ca.crt"+
		" --cert=/etc/kubernetes/pki/etcd/server.crt"+
		" --key=/etc/kubernetes/pki/etcd/server.key member list")
	if err != nil {
		t.Errorf("查 etcd 成员失败: %v\n输出：\n%s", err, members)
	} else if n := len(strings.Split(strings.TrimSpace(members), "\n")); n != 2 {
		t.Errorf("etcd 成员数 = %d，期望 2：\n%s", n, members)
	}

	// 客户端连的必须是那个稳定入口。
	kubeconfig, err := ssh(master2, "grep server: /etc/kubernetes/admin.conf")
	if err != nil {
		t.Fatalf("读 %s 上的 kubeconfig 失败: %v", master2.host, err)
	}
	if !strings.Contains(kubeconfig, lbEndpoint) {
		t.Errorf("kubeconfig 里的 server 不是稳定入口 %s，而是：\n%s", lbEndpoint, kubeconfig)
	}
}
