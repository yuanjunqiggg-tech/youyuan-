package main

// 入站消息幂等（落盘）。
//
// 修的问题：群里一条触发词消息被处理过一次之后，机器人重连/重启会让同一个
// 动作再执行一遍（点赞两次、指令打两次、回复发两条）。
//
// 原因不是本地有「待执行队列」——全仓没有落盘的消息队列，重连也不会重放。
// 真正缺的是幂等键：群消息链路唯一的去重是内存里的 yyQQRecent（10 分钟、
// 进程内），触发词自己的冷却/每日上限（yyQQTriggerState）同样只在内存里。
// 只要适配器把同一条 message_id 再推一次（重启/重新登录后同步最近的群
// 消息就是这种情况），三层内存态全为空，动作就重跑一遍。
//
// 所以这里补一个「已处理消息 id」集合：落盘、有界（500 条）、24 小时过期，
// 认领（claim）成功才允许后续副作用。先记账再执行，进程崩在中间也只会
// 少执行，不会多执行。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	yyQQClaimMax = 500            // 最多记 500 条；再老的先淘汰
	yyQQClaimTTL = 24 * time.Hour // 超过 24 小时的消息 id 视为可再次认领
)

type yyQQClaimStore struct {
	mu      sync.Mutex
	path    string
	loaded  bool
	items   map[string]int64 // 消息键 -> 首次处理时间（Unix 秒）
	order   []string         // 插入顺序，容量淘汰用
	diskErr string
}

func newYYQQClaimStore(path string) *yyQQClaimStore {
	return &yyQQClaimStore{path: strings.TrimSpace(path), items: map[string]int64{}}
}

// yyQQClaims 是线上实例；未初始化（如单测）时退化成纯内存去重，行为与旧版一致。
var yyQQClaims = newYYQQClaimStore("")

// yyQQClaimsInit 由 yyRegisterQQ 在启动时调用，把认领表落到 dataDir 下。
func yyQQClaimsInit(dataDir string) {
	path := ""
	if strings.TrimSpace(dataDir) != "" {
		path = filepath.Join(dataDir, "qq-inbound-claims.json")
	}
	yyQQClaims = newYYQQClaimStore(path)
}

// yyQQClaimKey 生成稳定的消息键。
//
// 不能用 yyQQMessageKey(cf)：它把 OneBot.BotID 也算进哈希，而 BotID 是收到
// 事件才学到的（只在内存里），重启初期为空 —— 同一条消息会被算成两个键，
// 落盘幂等正好在「重启」这个场景失效。这里用配置里本来就持久化的身份：
// 官方用 AppID，OneBot 用 WS 地址；再拼群号 + message_id。
func yyQQClaimKey(cf yyQQConfig, id string) string {
	account := strings.TrimSpace(cf.OneBot.URL)
	if cf.Adapter == "qqofficial" {
		account = strings.TrimSpace(cf.Official.AppID)
	}
	return cf.Adapter + "\x00" + account + "\x00" + yyQQGroupString(cf.GroupID) + "\x00" + id
}

func yyQQClaimMessage(cf yyQQConfig, id string) bool {
	if strings.TrimSpace(id) == "" {
		return true // 没有 message_id 的消息无处记账，维持旧行为
	}
	store := yyQQClaims
	if store == nil {
		return true
	}
	return store.claim(yyQQClaimKey(cf, id))
}

// claim 返回 true 表示这条消息第一次出现（调用方可以执行副作用），
// false 表示已经处理过（必须丢弃，避免重放）。
func (s *yyQQClaimStore) claim(key string) bool {
	if key == "" {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	now := time.Now()
	s.sweepLocked(now)
	if _, seen := s.items[key]; seen {
		return false
	}
	s.items[key] = now.Unix()
	s.order = append(s.order, key)
	s.trimLocked()
	s.persistLocked()
	return true
}

func (s *yyQQClaimStore) loadLocked() {
	if s.loaded {
		return
	}
	s.loaded = true
	if s.path == "" {
		return
	}
	raw, e := os.ReadFile(s.path)
	if e != nil {
		return // 首次运行没有文件，属于正常情况
	}
	var file struct {
		Items map[string]int64 `json:"items"`
	}
	if e := json.Unmarshal(raw, &file); e != nil {
		// 文件坏了就当没有历史：宁可保留旧行为，也不要因为读不动而放行全部重放。
		s.noteDiskErr("已处理消息表损坏，已重新开始记录：" + e.Error())
		return
	}
	for key, at := range file.Items {
		if key == "" || at <= 0 {
			continue
		}
		s.items[key] = at
	}
	keys := make([]string, 0, len(s.items))
	for key := range s.items {
		keys = append(keys, key)
	}
	// 载入后按时间排序，淘汰顺序与写入顺序一致。
	sort.Slice(keys, func(i, j int) bool {
		if s.items[keys[i]] == s.items[keys[j]] {
			return keys[i] < keys[j]
		}
		return s.items[keys[i]] < s.items[keys[j]]
	})
	s.order = keys
}

func (s *yyQQClaimStore) sweepLocked(now time.Time) {
	if len(s.order) == 0 {
		return
	}
	cutoff := now.Add(-yyQQClaimTTL).Unix()
	kept := s.order[:0]
	for _, key := range s.order {
		at, ok := s.items[key]
		if !ok {
			continue // items 里已经没了（被淘汰），order 一起清掉
		}
		if at < cutoff {
			delete(s.items, key)
			continue
		}
		kept = append(kept, key)
	}
	s.order = kept
}

func (s *yyQQClaimStore) trimLocked() {
	for len(s.order) > yyQQClaimMax {
		oldest := s.order[0]
		s.order = s.order[1:]
		delete(s.items, oldest)
	}
}

func (s *yyQQClaimStore) persistLocked() {
	if s.path == "" {
		return
	}
	raw, e := json.Marshal(map[string]any{"items": s.items})
	if e != nil {
		s.noteDiskErr(e.Error())
		return
	}
	tmp := s.path + ".tmp"
	if e = os.WriteFile(tmp, raw, 0600); e != nil {
		s.noteDiskErr(e.Error())
		return
	}
	if e = os.Rename(tmp, s.path); e != nil {
		s.noteDiskErr(e.Error())
		return
	}
	s.noteDiskErr("")
}

// noteDiskErr 落盘失败要能被看见：认领表写不进去，重连后就会回到「再执行一次」，
// 用户只看行为根本查不出原因。同一个错误只记一次日志，避免每条消息刷屏。
func (s *yyQQClaimStore) noteDiskErr(msg string) {
	if msg == s.diskErr {
		return
	}
	s.diskErr = msg
	if msg != "" {
		yyQQLog("已处理消息表落盘失败（重连后可能重复执行）：" + msg)
	}
}
