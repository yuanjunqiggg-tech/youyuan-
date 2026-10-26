package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ── 会话存储（AI 本地会话，长久化） ───────────────────────────────────

// aiAutoSession 自动保存的会话名（不显示在会话列表，打开页面自动读回）。
const aiAutoSession = "__auto__"

// aiSession 一个可保存/读档的 AI 会话。
type aiSession struct {
	Name     string    `json:"name"`
	Created  string    `json:"created"`
	PluginID string    `json:"plugin_id"`
	Messages []aiTurn  `json:"messages"`
}

// aiSessionStore 会话持久化：<dataDir>/ai_sessions/<name>.json。
type aiSessionStore struct {
	mu  sync.Mutex
	dir string
}

var sessionStore = &aiSessionStore{}

func (s *aiSessionStore) setPath(dataDir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dir = dataDir + "/ai_sessions"
	_ = os.MkdirAll(s.dir, 0755)
}

// safeName 过滤会话/存档名：只拦路径分隔符与控制字符，允许中文标点、冒号等（标题用）。
func safeName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "", false
	}
	for _, r := range name {
		if r == '/' || r == '\\' || r < 0x20 || r == 0x7f {
			return "", false
		}
	}
	return name, true
}

func (s *aiSessionStore) path(name string) string { return filepath.Join(s.dir, name+".json") }

// List 返回所有会话；pluginID 非空时只返回该插件所属的会话（会话隔离）。
func (s *aiSessionStore) List(pluginID string) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return []map[string]any{}
	}
	var out []map[string]any
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		if name == aiAutoSession {
			continue // 自动保存的会话不显示在列表
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var sess aiSession
		if json.Unmarshal(data, &sess) != nil {
			continue
		}
		if pluginID != "" && sess.PluginID != pluginID {
			continue
		}
		out = append(out, map[string]any{"name": sess.Name, "created": sess.Created, "plugin_id": sess.PluginID, "messages": len(sess.Messages)})
	}
	return out
}

func (s *aiSessionStore) Save(name, pluginID string, messages []aiTurn) error {
	name, ok := safeName(name)
	if !ok {
		return fmt.Errorf("名称含非法字符")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := aiSession{Name: name, Created: time.Now().Format("2006-01-02 15:04:05"), PluginID: pluginID, Messages: messages}
	data, _ := json.MarshalIndent(sess, "", "  ")
	return os.WriteFile(s.path(name), data, 0644)
}

func (s *aiSessionStore) Load(name string) (aiSession, error) {
	name, ok := safeName(name)
	if !ok {
		return aiSession{}, fmt.Errorf("名称含非法字符")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path(name))
	if err != nil {
		return aiSession{}, err
	}
	var sess aiSession
	if err := json.Unmarshal(data, &sess); err != nil {
		return aiSession{}, err
	}
	return sess, nil
}

func (s *aiSessionStore) Delete(name string) {
	name, ok := safeName(name)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = os.Remove(s.path(name))
}

// Rename 重命名一个会话（AI 可调用改标题）。
func (s *aiSessionStore) Rename(oldName, newName string) error {
	oldName, ok1 := safeName(oldName)
	newName, ok2 := safeName(newName)
	if !ok1 || !ok2 || newName == "" {
		return fmt.Errorf("名称含非法字符")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.path(oldName)); err != nil {
		return fmt.Errorf("会话不存在: %s", oldName)
	}
	if oldName == newName {
		return nil
	}
	oldPath, newPath := s.path(oldName), s.path(newName)
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("已存在同名会话: %s", newName)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}
	// 同步 JSON 内的 Name，保持文件名与内容一致
	if data, err := os.ReadFile(newPath); err == nil {
		var sess aiSession
		if json.Unmarshal(data, &sess) == nil {
			sess.Name = newName
			if d, err := json.MarshalIndent(sess, "", "  "); err == nil {
				_ = os.WriteFile(newPath, d, 0644)
			}
		}
	}
	return nil
}

// Recent 返回最近创建的一个会话（跳过自动保存的 __auto__）；pluginID 非空时只在该插件内找。
func (s *aiSessionStore) Recent(pluginID string) (aiSession, bool) {
	var best aiSession
	bestT := ""
	ok := false
	for _, it := range s.List(pluginID) {
		t, _ := it["created"].(string)
		n, _ := it["name"].(string)
		if n == "" || (!ok && t == "") || t > bestT {
			if sess, err := s.Load(n); err == nil {
				best = sess
				bestT = t
				ok = true
			}
		}
	}
	return best, ok
}

// ── 改动存档/读档（本次会话 AI 修改的插件文件快照） ───────────────────

// aiSnapshotStore 快照持久化：<dataDir>/ai_snapshots/<name>.json。内容 = 绝对路径 → 内容。
type aiSnapshotStore struct {
	mu  sync.Mutex
	dir string
}

var snapshotStore = &aiSnapshotStore{}

func (s *aiSnapshotStore) setPath(dataDir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dir = dataDir + "/ai_snapshots"
	_ = os.MkdirAll(s.dir, 0755)
}

func (s *aiSnapshotStore) path(name string) string { return filepath.Join(s.dir, name+".json") }

func (s *aiSnapshotStore) List() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return []map[string]any{}
	}
	var out []map[string]any
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var files map[string]string
		if json.Unmarshal(data, &files) != nil {
			continue
		}
		out = append(out, map[string]any{"name": strings.TrimSuffix(e.Name(), ".json"), "files": len(files)})
	}
	return out
}

// Save 把本次会话 AI 改过的插件文件内容做成快照。
func (s *aiSnapshotStore) Save(name string) error {
	name, ok := safeName(name)
	if !ok {
		return fmt.Errorf("名称含非法字符")
	}
	modified := aiModifiedList()
	if len(modified) == 0 {
		return fmt.Errorf("本次会话还没有 AI 修改过的文件")
	}
	snap := map[string]string{}
	for _, p := range modified {
		if !isPluginFilePath(p) {
			continue
		}
		data, err := os.ReadFile(p)
		if err == nil {
			snap[p] = string(data)
		}
	}
	if len(snap) == 0 {
		return fmt.Errorf("没有可存档的插件文件")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, _ := json.MarshalIndent(snap, "", "  ")
	return os.WriteFile(s.path(name), data, 0644)
}

// Restore 恢复一个快照（只恢复插件目录内文件）。
func (s *aiSnapshotStore) Restore(name string) (int, error) {
	name, ok := safeName(name)
	if !ok {
		return 0, fmt.Errorf("名称含非法字符")
	}
	s.mu.Lock()
	data, err := os.ReadFile(s.path(name))
	s.mu.Unlock()
	if err != nil {
		return 0, err
	}
	var files map[string]string
	if err := json.Unmarshal(data, &files); err != nil {
		return 0, err
	}
	n := 0
	for p, content := range files {
		if !isPluginFilePath(p) {
			continue // 只恢复插件文件
		}
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			continue
		}
		if os.WriteFile(p, []byte(content), 0644) == nil {
			n++
		}
	}
	return n, nil
}

func (s *aiSnapshotStore) Delete(name string) {
	name, ok := safeName(name)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = os.Remove(s.path(name))
}
