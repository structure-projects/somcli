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

// Package state 记录"哪个节点装了什么版本"。
//
// 没有它，install 只能是"每次都从头再来一遍"：脚本重复执行的副作用（追加写文件、
// 重复注册服务、重复解压覆盖）全落在用户头上，而 somcli 自己无从判断某个资源
// 是不是已经就位（E4）。
//
// 状态是**辅助判据**而不是唯一判据：文件读不出来或者内容坏了，一律降级为
// "按无状态执行"（顶多多装一遍），绝不因为一个记账文件而让安装失败 ——
// 那样等于给引擎新加了一种自己造出来的故障模式。
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/structure-projects/somcli/pkg/utils"
)

// fileVersion 是状态文件的结构版本，将来改格式时用它区分。
const fileVersion = 1

// LocalHost 是"在操作机本地实施"这一目标在状态里的键。
// 用空串而不是 "localhost"：hosts 未声明与显式写 localhost 是两种配置，
// 前者压根没有节点概念，硬塞一个主机名会让 somcli status 显示出用户没写过的主机。
const LocalHost = ""

// Record 一条安装记录。
type Record struct {
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	Host      string    `json:"host"`
	Method    string    `json:"method"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Target 返回这条记录的可读目标名。
func (r Record) Target() string {
	if r.Host == LocalHost {
		return "(local)"
	}
	return r.Host
}

type stateFile struct {
	Version int      `json:"version"`
	Records []Record `json:"records"`
}

// Store 是状态文件的内存视图。非并发安全：调用方（installer）在资源之间串行更新。
type Store struct {
	path    string
	records []Record
}

// Path 返回状态文件路径。跟着 workdir 走，所以 --workdir 换目录等于换一套状态，
// 用例之间天然隔离。
func Path() string {
	return filepath.Join(utils.GetWorkDir(), "state.json")
}

// Load 读状态文件。任何读取或解析失败都降级为空状态并出声，不返回错误。
func Load() *Store {
	store := &Store{path: Path()}

	data, err := os.ReadFile(store.path)
	if errors.Is(err, fs.ErrNotExist) {
		return store
	}
	if err != nil {
		utils.PrintWarning("状态文件 %s 读不出来，本次按无状态执行: %v", store.path, err)
		return store
	}

	var parsed stateFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		utils.PrintWarning("状态文件 %s 已损坏，本次按无状态执行（已安装的资源会被重新实施）: %v",
			store.path, err)
		return store
	}
	store.records = parsed.Records
	return store
}

// Installed 判断某个目标上是否已记录该资源的该版本。
// 版本不同即视为未安装 —— 这正是 version 变更后重新实施的判据（SC-E09）。
func (s *Store) Installed(name, version, host string) bool {
	for _, rec := range s.records {
		if rec.Name == name && rec.Host == host {
			return rec.Version == version
		}
	}
	return false
}

// InstalledVersion 返回某目标上已记录的版本，未记录则返回空串。
func (s *Store) InstalledVersion(name, host string) string {
	for _, rec := range s.records {
		if rec.Name == name && rec.Host == host {
			return rec.Version
		}
	}
	return ""
}

// Put 写入一条记录：同一 (name, host) 只保留最新一条，换版本是覆盖而非追加。
func (s *Store) Put(rec Record) {
	rec.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	for i, old := range s.records {
		if old.Name == rec.Name && old.Host == rec.Host {
			s.records[i] = rec
			return
		}
	}
	s.records = append(s.records, rec)
}

// Forget 删除某目标上的记录。卸载后必须删：留着的话重装会被幂等判据跳过，
// 用户会看到"已安装，跳过"而机器上什么都没有。
func (s *Store) Forget(name, host string) {
	kept := s.records[:0]
	for _, rec := range s.records {
		if rec.Name == name && rec.Host == host {
			continue
		}
		kept = append(kept, rec)
	}
	s.records = kept
}

// Records 返回按资源名、目标排序的记录副本，供 somcli status 输出。
func (s *Store) Records() []Record {
	out := make([]Record, len(s.records))
	copy(out, s.records)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Host < out[j].Host
	})
	return out
}

// Save 落盘。先写临时文件再 rename：中途被打断也不会留下半截 JSON，
// 而半截 JSON 下次会被判为"损坏"从而丢掉全部记录。
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("创建状态文件目录失败: %w", err)
	}

	data, err := json.MarshalIndent(stateFile{Version: fileVersion, Records: s.records}, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化状态失败: %w", err)
	}
	data = append(data, '\n')

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("写状态文件失败: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("提交状态文件失败: %w", err)
	}
	return nil
}
