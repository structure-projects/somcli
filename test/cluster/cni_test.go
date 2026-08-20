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
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestSC_K06_K07_CNI 配置里选的网络插件必须真的是装上去的那个。
//
// 与 single_node / multi_node 的区别在于断言对象：那两条只问"节点 Ready 了吗"，
// 而 Ready 只要有任意一个 CNI 就成立。这里要问的是 cni: 这个键有没有被消费 ——
// 写 calico 装出 flannel，节点一样 Ready，一样全绿。
//
// 判据取三层：插件自己的 Pod 起来了、节点 Ready、Pod 拿到配置网段里的地址。
func TestSC_K06_K07_CNI(t *testing.T) {
	cases := []struct {
		name string
		cni  string
		// podLabel 是该插件自身 DaemonSet 的标志性 Pod 名前缀。
		// 换成看 DaemonSet 名字也行，但 Pod 名前缀在两个插件上写法一致。
		podPrefix string
	}{
		{name: "SC-K07_flannel", cni: "flannel", podPrefix: "kube-flannel-ds"},
		{name: "SC-K06_calico", cni: "calico", podPrefix: "calico-node"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(func() {
				if t.Failed() {
					dumpNodeLogs()
				}
			})

			resetCluster(t, master, worker)

			cfg := clusterConfig(t, "sc-cni-"+tc.cni, "1.28.2", "containerd", tc.cni, master)
			code, out := runSomcli(t, "cluster", "create", "-f", cfg)
			if code != 0 {
				t.Fatalf("cni=%s 的 cluster create 退出码 = %d\n输出：\n%s", tc.cni, code, out)
			}

			waitFor(t, tc.cni+" 自己的 Pod 处于 Running", 5*time.Minute, func() (bool, string) {
				got, err := kubectl(t, "get", "pods", "-A", "--no-headers")
				if err != nil {
					return false, err.Error()
				}
				for _, line := range strings.Split(strings.TrimSpace(got), "\n") {
					fields := strings.Fields(line)
					if len(fields) < 4 {
						continue
					}
					if strings.HasPrefix(fields[1], tc.podPrefix) && fields[3] == "Running" {
						return true, got
					}
				}
				return false, got
			})

			waitFor(t, "节点 Ready", 5*time.Minute, func() (bool, string) {
				got, err := kubectl(t, "get", "nodes", "--no-headers")
				if err != nil {
					return false, err.Error()
				}
				for _, f := range strings.Fields(got) {
					if f == "Ready" {
						return true, got
					}
				}
				return false, got
			})

			// Pod 地址落在配置的网段里，才说明清单里的 CIDR 被改成了配置里的值。
			// calico 的清单默认是 192.168.0.0/16 且那两行是注释掉的，
			// 没改对的话地址会落在 192.168 上而不是 10.244。
			podName := "cni-probe-" + tc.cni
			applyManifest(t, fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: %s
spec:
  nodeName: %s
  tolerations:
    - operator: "Exists"
  containers:
    - name: probe
      image: busybox:1.36
      command: ["sleep", "600"]
`, podName, master.host))

			var podIP string
			waitFor(t, "探针 Pod 拿到地址", 4*time.Minute, func() (bool, string) {
				got, err := kubectl(t, "get", "pod", podName, "-o", "jsonpath={.status.podIP}")
				if err != nil {
					return false, err.Error()
				}
				podIP = strings.TrimSpace(got)
				return podIP != "", got
			})
			if !strings.HasPrefix(podIP, "10.244.") {
				t.Errorf("cni=%s 时 Pod 地址 %s 不在配置的 10.244.0.0/16 内", tc.cni, podIP)
			}
		})
	}
}
