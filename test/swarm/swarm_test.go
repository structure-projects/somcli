//go:build swarm

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
package swarm

import (
	"strings"
	"testing"
	"time"
)

// TestSC_S01_S04_Lifecycle 一次装出 2 manager + 1 worker 的 swarm，再拆掉。
//
// 四个场景放在一条用例里是因为它们共享同一个昂贵前提（一套装好的 swarm），
// 而且拆集群只能拆一个已经装好的 —— 分开写第二条得把第一条再做一遍。
func TestSC_S01_S04_Lifecycle(t *testing.T) {
	t.Cleanup(func() {
		if t.Failed() {
			dumpNodeLogs()
		}
	})

	resetSwarm(t, allNodes...)

	cfg := swarmConfig(t, "sc-s01", "", manager1, manager2, worker1)
	if code, out := runSomcli(t, "cluster", "create", "-f", cfg); code != 0 {
		t.Fatalf("cluster create 退出码 = %d\n输出：\n%s", code, out)
	}

	t.Run("SC-S01 manager 初始化完成且是 Leader", func(t *testing.T) {
		// 判据不取"init 退了 0"：swarm init 失败照样可能留下一个半死的状态。
		// 取 manager 自己报的 Leader —— 没有 Leader 的 swarm 什么服务都调度不了。
		got := nodeLs(t, "{{.Hostname}} {{.ManagerStatus}}")
		if !strings.Contains(got, manager1.host+" Leader") {
			t.Errorf("%s 不是 Leader：\n%s", manager1.host, got)
		}
	})

	t.Run("SC-S02 worker 加入", func(t *testing.T) {
		got := nodeLs(t, "{{.Hostname}} {{.Status}} {{.ManagerStatus}}")
		if !strings.Contains(got, worker1.host+" Ready") {
			t.Errorf("worker %s 没有 Ready：\n%s", worker1.host, got)
		}
		// worker 不该有 manager 状态：角色写错的话它会作为 manager 加进来，
		// 而 `docker node ls` 里"多了一行"看不出这个区别。
		for _, line := range strings.Split(got, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), worker1.host+" ") &&
				(strings.Contains(line, "Reachable") || strings.Contains(line, "Leader")) {
				t.Errorf("配的是 worker，却作为 manager 加进来了：%s", line)
			}
		}
	})

	t.Run("SC-S03 第二个 manager 加入", func(t *testing.T) {
		waitFor(t, "第二个 manager 变 Reachable", 2*time.Minute, func() (bool, string) {
			got := nodeLs(t, "{{.Hostname}} {{.ManagerStatus}}")
			return strings.Contains(got, manager2.host+" Reachable"), got
		})
	})

	t.Run("集群真的能调度服务", func(t *testing.T) {
		// 前面几条只看 `docker node ls` 的字面。这条验的是这套 swarm 能干活：
		// overlay 网络与 raft 都有问题时，node ls 照样是好的，而服务永远起不来。
		if out, err := ssh(manager1, "docker service create --name s01-probe --replicas 3 --detach "+
			"busybox:1.36 sleep 600"); err != nil {
			t.Fatalf("创建服务失败: %v\n输出：\n%s", err, out)
		}
		waitFor(t, "3 个副本都跑起来", 4*time.Minute, func() (bool, string) {
			out, err := ssh(manager1, "docker service ls --filter name=s01-probe --format '{{.Replicas}}'")
			if err != nil {
				return false, err.Error()
			}
			return strings.TrimSpace(out) == "3/3", out
		})
	})

	t.Run("SC-S04 集群移除", func(t *testing.T) {
		// --force 跳过交互确认：没有终端时 AskForConfirmation 读到 EOF 会当成"否"。
		if code, out := runSomcli(t, "cluster", "remove", "-f", cfg, "--force"); code != 0 {
			t.Fatalf("cluster remove 退出码 = %d\n输出：\n%s", code, out)
		}
		// 判据是每台节点自己都不在 swarm 里了。只查 manager 的话，
		// 一个"只让 manager 退出"的实现同样能过，而 worker 上还留着旧集群的状态。
		for _, n := range allNodes {
			if state := swarmState(t, n); state != "inactive" {
				t.Errorf("%s 的 swarm 状态是 %s，期望 inactive", n.host, state)
			}
		}
	})
}

// TestSC_S05_S06_Scale swarm 的扩容与缩容。
//
// 先装一个不含 worker 的 swarm，再把 worker 加进来、又摘出去。
func TestSC_S05_S06_Scale(t *testing.T) {
	t.Cleanup(func() {
		if t.Failed() {
			dumpNodeLogs()
		}
	})

	resetSwarm(t, allNodes...)

	base := swarmConfig(t, "sc-s05", "", manager1, manager2)
	if code, out := runSomcli(t, "cluster", "create", "-f", base); code != 0 {
		t.Fatalf("cluster create 退出码 = %d\n输出：\n%s", code, out)
	}

	// 扩容用的配置多写一台。要加的节点必须先写进配置里 —— 凭据与角色都在那儿。
	scaled := swarmConfig(t, "sc-s05", "", manager1, manager2, worker1)

	t.Run("SC-S05 扩容：新增一台 worker", func(t *testing.T) {
		if code, out := runSomcli(t, "cluster", "add-node", "-f", scaled, "--node", worker1.host); code != 0 {
			t.Fatalf("cluster add-node 退出码 = %d\n输出：\n%s", code, out)
		}

		waitFor(t, "新节点 Ready", 3*time.Minute, func() (bool, string) {
			got := nodeLs(t, "{{.Hostname}} {{.Status}}")
			return strings.Contains(got, worker1.host+" Ready"), got
		})

		// 判据不止"多了一行"：新节点上得真的能落任务。docker 装上了但
		// overlay 网络起不来时，节点是 Ready 的，而任务会一直 Pending。
		if out, err := ssh(manager1, "docker service create --name s05-probe --detach "+
			"--constraint node.hostname=="+worker1.host+" busybox:1.36 sleep 600"); err != nil {
			t.Fatalf("创建服务失败: %v\n输出：\n%s", err, out)
		}
		waitFor(t, "副本落在新节点上并跑起来", 4*time.Minute, func() (bool, string) {
			out, err := ssh(manager1, "docker service ps s05-probe --format '{{.Node}} {{.CurrentState}}'")
			if err != nil {
				return false, err.Error()
			}
			return strings.Contains(out, worker1.host) && strings.Contains(out, "Running"), out
		})
	})

	t.Run("SC-S06 缩容：摘掉这台 worker", func(t *testing.T) {
		if code, out := runSomcli(t, "cluster", "remove-node", "-f", scaled,
			"--node", worker1.host, "--force"); code != 0 {
			t.Fatalf("cluster remove-node 退出码 = %d\n输出：\n%s", code, out)
		}

		got := nodeLs(t, "{{.Hostname}}")
		if strings.Contains(got, worker1.host) {
			t.Errorf("摘除之后 docker node ls 里仍有 %s：\n%s", worker1.host, got)
		}
		// 剩下两台不该被牵连。
		for _, n := range []node{manager1, manager2} {
			if !strings.Contains(got, n.host) {
				t.Errorf("摘一台 worker 却把 %s 也弄没了：\n%s", n.host, got)
			}
		}
		// 被摘的机器自己也要退出：只在 manager 上删记录的话，那台机器还以为
		// 自己在集群里，下次加回来会撞上"already part of a swarm"。
		if state := swarmState(t, worker1); state != "inactive" {
			t.Errorf("%s 的 swarm 状态是 %s，期望 inactive", worker1.host, state)
		}
	})
}
