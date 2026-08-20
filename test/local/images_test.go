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
package local

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeDockerDir 造一个假 docker：镜像名里含 failMark 的操作失败，其余成功。
//
// 不用真 docker：要验的是"逐张镜像的失败有没有被汇总、有没有影响退出码"，
// 这与镜像真的拉没拉下来无关，而真 docker 还会让用例依赖网络与守护进程。
func fakeDockerDir(t *testing.T, logPath, failMark string) string {
	t.Helper()

	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %q
for a in "$@"; do
  case "$a" in
    *%s*) echo "boom: $a" >&2; exit 1 ;;
  esac
done
exit 0
`, logPath, failMark)

	return fakeBinDir(t, map[string]string{"docker": script})
}

// writeImageList 落一份镜像清单（每行 name:tag，README 里的 image-list.txt 就是这种）。
func writeImageList(t *testing.T, images ...string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "image-list.txt")
	if err := os.WriteFile(path, []byte(strings.Join(images, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("写镜像清单失败: %v", err)
	}
	return path
}

// F14：pull 里有镜像失败就必须非 0 退出，并把失败的镜像逐条列出来。
//
// 原实现对每张失败只 Warnf 一句然后 continue，最后 return nil：
// 一张都没拉到也照样 exit 0，调用方（CI、离线打包脚本）完全感知不到。
func TestF14_PullAggregatesFailuresAndExitsNonZero(t *testing.T) {
	log := filepath.Join(t.TempDir(), "docker.log")
	bin := fakeDockerDir(t, log, "broken")
	list := writeImageList(t, "library/nginx:1.25", "org/broken:1.0")

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin},
		"images", "pull", "-f", list)
	if code == 0 {
		t.Fatalf("有镜像拉取失败却退出 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "org/broken:1.0") {
		t.Fatalf("错误信息没有点出失败的镜像，输出：\n%s", out)
	}
	// 成功的那张也得真的被拉过：汇总失败不等于遇错即停
	if got := readFile(t, log); !strings.Contains(got, "library/nginx:1.25") {
		t.Fatalf("失败一张就不再处理后面的镜像了，docker 调用记录：\n%s", got)
	}
}

// F14：有失败就不写镜像清单。
//
// 清单是"这些镜像已在本地"的凭据，下游 export/push 照着它干活。
// 把没拉下来的镜像写进去，等于把失败伪装成成功交给下一步。
func TestF14_PullSkipsImageListOnFailure(t *testing.T) {
	log := filepath.Join(t.TempDir(), "docker.log")
	bin := fakeDockerDir(t, log, "broken")
	list := writeImageList(t, "library/nginx:1.25", "org/broken:1.0")
	manifest := filepath.Join(t.TempDir(), "pulled.yaml")

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin},
		"images", "pull", "-f", list, "-o", manifest)
	if code == 0 {
		t.Fatalf("有镜像拉取失败却退出 0，输出：\n%s", out)
	}
	if _, err := os.Stat(manifest); err == nil {
		t.Fatalf("有镜像失败却写出了清单 %s：\n%s", manifest, readFile(t, manifest))
	}
}

// F14：全都成功时才写清单，且退出 0 —— 汇总逻辑不能把正常路径带坏。
func TestF14_PullWritesImageListWhenAllSucceed(t *testing.T) {
	log := filepath.Join(t.TempDir(), "docker.log")
	bin := fakeDockerDir(t, log, "no-such-mark")
	list := writeImageList(t, "library/nginx:1.25", "library/redis:7")
	manifest := filepath.Join(t.TempDir(), "pulled.yaml")

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin},
		"images", "pull", "-f", list, "-o", manifest)
	if code != 0 {
		t.Fatalf("全部成功却退出码 = %d，输出：\n%s", code, out)
	}

	got := readFile(t, manifest)
	for _, want := range []string{"nginx", "redis"} {
		if !strings.Contains(got, want) {
			t.Fatalf("清单里缺 %s：\n%s", want, got)
		}
	}
}

// F14：push 同样要聚合失败并影响退出码。
func TestF14_PushAggregatesFailures(t *testing.T) {
	log := filepath.Join(t.TempDir(), "docker.log")
	bin := fakeDockerDir(t, log, "broken")
	list := writeImageList(t, "library/nginx:1.25", "org/broken:1.0")

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin},
		"images", "push", "-i", list)
	if code == 0 {
		t.Fatalf("有镜像推送失败却退出 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "org/broken:1.0") {
		t.Fatalf("错误信息没有点出失败的镜像，输出：\n%s", out)
	}
}

// F14：docker 根本不在 PATH 上时，pull 必须失败 —— 这是最容易被 Warnf 吞掉的一种。
func TestF14_PullFailsWhenDockerMissing(t *testing.T) {
	emptyBin := t.TempDir()
	list := writeImageList(t, "library/nginx:1.25")

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + emptyBin},
		"images", "pull", "-f", list)
	if code == 0 {
		t.Fatalf("没有 docker 却退出 0，输出：\n%s", out)
	}
}
