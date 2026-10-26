package main

import (
	"bot-apk/building"
	"bot-apk/state"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

type yyMultiImportCheckpoint struct {
	Version      int    `json:"version"`
	SourceSHA256 string `json:"source_sha256"`
	SplitBots    int    `json:"split_bots"`
	LayoutSHA256 string `json:"layout_sha256"`
	Completed    []int  `json:"completed_units"`
}

// A user stop or lost connection is resumable, not a failed import.
// Genuine execution errors retain their exact message and failed state.
func yyImportInterruptedOutcome(tc *state.TaskController, disconnected bool, err error) (string, string) {
	if disconnected {
		return "paused", "连接中断，已保存导入断点；重新连接后可继续"
	}
	if !tc.Running() {
		return "paused", "已停止并保存导入断点，可继续"
	}
	if err != nil {
		return "failed", err.Error()
	}
	return "", ""
}

func yySelectImportWorkers(owner *BotManager, bots []*BotManager, p map[string]any) ([]*BotManager, error) {
	if owner == nil {
		return nil, fmt.Errorf("当前账号未连接")
	}
	selected, explicit := p["bot_names"]
	names := map[string]bool{}
	if explicit {
		rows, ok := selected.([]any)
		if !ok {
			return nil, fmt.Errorf("bot_names必须是游戏名称数组")
		}
		for _, row := range rows {
			name, ok := row.(string)
			if !ok || name == "" {
				return nil, fmt.Errorf("机器人名称无效")
			}
			names[name] = true
		}
		if len(names) == 0 {
			return nil, fmt.Errorf("请至少选择一台导入机器人")
		}
	}
	server := owner.ServerCode()
	result := []*BotManager{}
	for _, bm := range bots {
		if bm == nil || bm.ServerCode() != server {
			continue
		}
		if !explicit || names[bm.display] {
			result = append(result, bm)
			delete(names, bm.display)
		}
	}
	if len(names) > 0 {
		return nil, fmt.Errorf("所选机器人未在当前服务器就绪，请刷新机器人列表")
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("当前服务器没有可用的导入机器人")
	}
	return result, nil
}

func yyImportSourceHash(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func yyImportLayoutHash(res *building.SplitResult) string {
	h := sha256.New()
	enc := json.NewEncoder(h)
	for _, u := range res.Units {
		// Sort copies: never mutate the structure shared by running workers.
		copy := *u
		copy.Chunks = append([][2]int(nil), u.Chunks...)
		copy.Positions = append([][3]int(nil), u.Positions...)
		copy.Skipped = append([][3]int(nil), u.Skipped...)
		sort.Slice(copy.Chunks, func(i, j int) bool {
			a, b := copy.Chunks[i], copy.Chunks[j]
			if a[0] != b[0] {
				return a[0] < b[0]
			}
			return a[1] < b[1]
		})
		less := func(a, b [3]int) bool {
			for k := 0; k < 3; k++ {
				if a[k] != b[k] {
					return a[k] < b[k]
				}
			}
			return false
		}
		sort.Slice(copy.Positions, func(i, j int) bool { return less(copy.Positions[i], copy.Positions[j]) })
		sort.Slice(copy.Skipped, func(i, j int) bool { return less(copy.Skipped[i], copy.Skipped[j]) })
		_ = enc.Encode(copy)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func yyLoadMultiResume(id, path string, bots int) (*state.Checkpoint, yyMultiImportCheckpoint, error) {
	hash, e := yyImportSourceHash(path)
	meta := yyMultiImportCheckpoint{Version: 1, SourceSHA256: hash, SplitBots: bots}
	if e != nil {
		return nil, meta, e
	}
	if id == "" {
		return &state.Checkpoint{TaskID: uuid.NewString(), Type: "import_multi", Status: "running", Name: shortName(path), CreatedAt: time.Now().Unix()}, meta, nil
	}
	cp, e := state.LoadCheckpoint(id)
	if e != nil {
		return nil, meta, e
	}
	var saved yyMultiImportCheckpoint
	if e = json.Unmarshal(cp.Data, &saved); e != nil || saved.Version != 1 || saved.SplitBots < 1 {
		return nil, meta, fmt.Errorf("多机器人断点格式不兼容，原记录已保留，请重新设置导入")
	}
	if saved.SourceSHA256 != hash {
		return nil, meta, fmt.Errorf("建筑文件已改变，不能沿用旧断点；请作为新任务导入")
	}
	return cp, saved, nil
}

func yyImportRegion(p map[string]any) [][6]int {
	if !strings.HasSuffix(strings.ToLower(getString(p, "path", "")), ".mcworld") {
		return nil
	}
	return [][6]int{{getInt(p, "x1", 0), getInt(p, "y1", -64), getInt(p, "z1", 0), getInt(p, "x2", 256), getInt(p, "y2", 256), getInt(p, "z2", 256)}}
}
