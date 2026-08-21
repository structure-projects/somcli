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

// TestSC_K08_VersionMatrix 三条版本线各装一遍：1.28 / 1.29 / 1.30。
//
// 为什么不是"装一个版本就够了"：版本号在四个地方各走一条路 —— 二进制下载地址
// （configs/k8s/kubernetes.yaml 里的 dl.k8s.io/v{{.Version}}）、`kubeadm init
// --kubernetes-version`、kubelet 自己的版本、以及控制面镜像的 tag。
// 这四处只要有一处没接上配置，装出来的就是"另一个版本的集群"，而集群照样能 Ready，
// 只装一个版本的话永远看不出来。
//
// 所以判据不是"装成了"，而是"装出来的三处版本都与配置里写的一致"。
//
// 每条都用单 master：这一组要验的是版本，不是拓扑（拓扑由 SC-K01/K09 验）。
// 单 master 是最便宜的一次完整安装，三遍才装得下每晚那点时间预算。
func TestSC_K08_VersionMatrix(t *testing.T) {
	t.Cleanup(func() {
		if t.Failed() {
			dumpNodeLogs()
		}
	})

	// 1.28 是别处用例的默认版本，这里仍然跑一遍：三条并列才看得出"换版本"这件事本身
	// 有没有生效 —— 只跑 1.29/1.30 的话，一个把版本写死成 1.30 的实现能过一半。
	for _, version := range []string{"1.28.2", "1.29.8", "1.30.4"} {
		t.Run(version, func(t *testing.T) {
			resetCluster(t, master, worker)

			cfg := clusterConfig(t, "sc-k08", version, "containerd", "", master)
			if code, out := runSomcli(t, "cluster", "create", "-f", cfg); code != 0 {
				t.Fatalf("cluster create 退出码 = %d\n输出：\n%s", code, out)
			}

			waitFor(t, "节点 Ready", 6*time.Minute, func() (bool, string) {
				out, err := kubectl(t, "get", "nodes", "--no-headers")
				if err != nil {
					return false, err.Error()
				}
				for _, f := range strings.Fields(out) {
					if f == "Ready" {
						return true, out
					}
				}
				return false, out
			})

			want := "v" + version

			// 节点上的 kubeadm。装错版本的二进制时，后面两处也会跟着错，
			// 但先单独查它一次：这一处错说明下载地址没用上配置里的版本。
			out, err := ssh(master, "kubeadm version -o short")
			if err != nil {
				t.Fatalf("查 kubeadm 版本失败: %v\n输出：\n%s", err, out)
			}
			if got := strings.TrimSpace(out); got != want {
				t.Errorf("节点上的 kubeadm 是 %s，配置写的是 %s", got, want)
			}

			// kubelet 报给 apiserver 的版本。
			out, err = kubectl(t, "get", "nodes", "-o",
				"jsonpath='{.items[0].status.nodeInfo.kubeletVersion}'")
			if err != nil {
				t.Fatalf("查 kubelet 版本失败: %v\n输出：\n%s", err, out)
			}
			if got := strings.Trim(strings.TrimSpace(out), "'"); got != want {
				t.Errorf("kubelet 版本是 %s，配置写的是 %s", got, want)
			}

			// 控制面自身的版本。这一处单独查是因为它由 `kubeadm init
			// --kubernetes-version` 决定，与二进制版本是两条独立的路：
			// 那个参数传空或传错时，kubeadm 会按自己的默认拉一套控制面镜像，
			// 于是 kubelet 是 1.30 而 apiserver 是别的版本，集群照样 Ready。
			out, err = kubectl(t, "get", "pod", "-n", "kube-system",
				"-l", "component=kube-apiserver", "-o",
				"jsonpath='{.items[0].spec.containers[0].image}'")
			if err != nil {
				t.Fatalf("查 apiserver 镜像失败: %v\n输出：\n%s", err, out)
			}
			if image := strings.Trim(strings.TrimSpace(out), "'"); !strings.HasSuffix(image, ":"+want) {
				t.Errorf("apiserver 镜像是 %s，期望以 :%s 结尾", image, want)
			}

			// 版本对了还要能干活：Pod 拿到 Pod 网段里的地址才说明这套版本组合
			// （kubelet + containerd + CNI）真的配得上。
			applyManifest(t, `apiVersion: v1
kind: Pod
metadata:
  name: k08-probe
  namespace: default
spec:
  # 单 master 集群上控制面节点带 NoSchedule 污点，不容忍的话 Pod 永远 Pending。
  tolerations:
    - operator: Exists
  containers:
    - name: probe
      image: busybox:1.36
      command: ["sleep", "600"]
`)
			waitFor(t, "探针 Pod 拿到 Pod 网段的地址", 4*time.Minute, func() (bool, string) {
				out, err := kubectl(t, "get", "pod", "k08-probe", "-o",
					"jsonpath='{.status.phase} {.status.podIP}'")
				if err != nil {
					return false, err.Error()
				}
				got := strings.Trim(strings.TrimSpace(out), "'")
				return strings.HasPrefix(got, "Running ") && strings.Contains(got, " 10.244."), got
			})
		})
	}
}
