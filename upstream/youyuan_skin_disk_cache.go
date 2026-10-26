package main

// 皮肤磁盘缓存：把内核已经收到的皮肤纹理落到数据目录。
//
// 为什么需要它：皮肤只从 PlayerList / PlayerSkin 包来，进程一重启，
// playerInfoMgr 的内存表就空了。箱子重启、账号掉线重连、或者服务器这次
// 没把某个玩家放进 PlayerList，账号页的立体预览就会一直停在「皮肤加载中」。
// 收到一次就写盘，之后无论在线与否、重启与否都能立刻给出同一个人。
//
// 与 youyuan_skin_cache.go 的区别：那份是「按名字/UUID 去第三方查头像」的
// 代理缓存；这份是「内核自己收到的皮肤」的权威落盘，不经过第三方。

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type yySkinDiskRecord struct {
	Name    string `json:"name"`
	Width   uint32 `json:"width"`
	Height  uint32 `json:"height"`
	Updated int64  `json:"updated"`
}

type yySkinDiskJob struct {
	Name   string
	Data   []byte
	Width  uint32
	Height uint32
}

var (
	yySkinDiskRoot   string
	yySkinDiskOnce   sync.Once
	yySkinDiskQueue  = make(chan yySkinDiskJob, 64)
	yySkinDiskDone   = map[string]bool{}
	yySkinDiskMu     sync.Mutex
	yySkinDiskQueued bool
)

func yySkinDiskInit(dataDir string) {
	yySkinDiskOnce.Do(func() {
		if strings.TrimSpace(dataDir) == "" {
			return
		}
		yySkinDiskRoot = filepath.Join(dataDir, "插件数据文件", "皮肤缓存")
		_ = os.MkdirAll(yySkinDiskRoot, 0755)
		yySkinDiskQueued = true
		go yySkinDiskLoop()
	})
}

func yySkinDiskLoop() {
	for job := range yySkinDiskQueue {
		yySkinDiskWrite(job)
	}
}

// 同一个玩家在一次运行里只写一次；皮肤换了会走 handlePlayerSkin，届时内容会覆盖。
func yySkinCacheEnqueue(name string, data []byte, width, height uint32) {
	name = strings.TrimSpace(name)
	if yySkinDiskRoot == "" || name == "" || len(data) == 0 || width == 0 || height == 0 {
		return
	}
	if uint64(len(data)) != uint64(width)*uint64(height)*4 {
		return
	}
	key := yySkinDiskKey(name)
	yySkinDiskMu.Lock()
	if yySkinDiskDone[key] {
		// 同一次运行里已经写过一次，但皮肤可能变了（机器人进服后内核才发来真正的
		// 纹理，或者注册时落盘的那张是旧皮肤）。内容一致才跳过，否则放行重写，
		// 否则账号页会一直显示注册即入库时落盘的旧纹理。
		if existing, ew, eh, ok := yySkinCacheGet(name); ok && ew == width && eh == height && bytes.Equal(existing, data) {
			yySkinDiskMu.Unlock()
			return
		}
	}
	yySkinDiskDone[key] = true
	yySkinDiskMu.Unlock()
	select {
	case yySkinDiskQueue <- yySkinDiskJob{Name: name, Data: append([]byte(nil), data...), Width: width, Height: height}:
	default:
		// 队列满就丢掉这次写盘，内存里仍然可用，不影响当前显示。
	}
}

func yySkinDiskKey(name string) string {
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(name))))
	return hex.EncodeToString(sum[:])
}

// 皮肤缓存是"只增不减"的写盘入口，也是箱子被写满的最可能来源之一。
// 这里给硬上限：文件数 400、总量 200MB，超了按最旧时间淘汰，
// 保证它永远不会把磁盘吃干——磁盘写满会让 sshd 都起不来。
const (
	yySkinDiskMaxFiles = 400
	yySkinDiskMaxBytes = 200 << 20
)

// yySkinDiskTrim 在每次写盘前跑一遍，超限就删最旧的 .rgba/.json 对。
// 用 os.ReadDir 与 ModTime 而不是 syscall.Statfs：后者在 Windows 上不可用，
// 会让本机测试直接编译失败。
func yySkinDiskTrim() {
	entries, err := os.ReadDir(yySkinDiskRoot)
	if err != nil {
		return
	}
	type item struct {
		name string
		size int64
		mod  time.Time
	}
	items := make([]item, 0, len(entries))
	var total int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		items = append(items, item{name: entry.Name(), size: info.Size(), mod: info.ModTime()})
		total += info.Size()
	}
	if len(items) <= yySkinDiskMaxFiles && total <= yySkinDiskMaxBytes {
		return
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mod.Before(items[j].mod) })
	excess := total - yySkinDiskMaxBytes
	for _, it := range items {
		if len(items) <= 0 || (excess <= 0 && len(items) <= yySkinDiskMaxFiles) {
			break
		}
		_ = os.Remove(filepath.Join(yySkinDiskRoot, it.name))
		excess -= it.size
		items = items[1:]
		// 成对清理：删掉 .rgba 时同名的 .json 一起走，避免留孤儿元数据。
		base := strings.TrimSuffix(it.name, filepath.Ext(it.name))
		_ = os.Remove(filepath.Join(yySkinDiskRoot, base+".json"))
	}
}

func yySkinDiskWrite(job yySkinDiskJob) {
	yySkinDiskTrim()
	key := yySkinDiskKey(job.Name)
	if err := os.WriteFile(filepath.Join(yySkinDiskRoot, key+".rgba"), job.Data, 0644); err != nil {
		yySkinDiskMu.Lock()
		delete(yySkinDiskDone, key)
		yySkinDiskMu.Unlock()
		return
	}
	meta, _ := json.Marshal(yySkinDiskRecord{Name: job.Name, Width: job.Width, Height: job.Height, Updated: time.Now().Unix()})
	_ = os.WriteFile(filepath.Join(yySkinDiskRoot, key+".json"), meta, 0644)
}

// yySkinCacheGet 读磁盘缓存；内存未命中时用。
func yySkinCacheGet(name string) ([]byte, uint32, uint32, bool) {
	if yySkinDiskRoot == "" {
		return nil, 0, 0, false
	}
	key := yySkinDiskKey(name)
	raw, err := os.ReadFile(filepath.Join(yySkinDiskRoot, key+".json"))
	if err != nil {
		return nil, 0, 0, false
	}
	var rec yySkinDiskRecord
	if json.Unmarshal(raw, &rec) != nil || rec.Width == 0 || rec.Height == 0 {
		return nil, 0, 0, false
	}
	data, err := os.ReadFile(filepath.Join(yySkinDiskRoot, key+".rgba"))
	if err != nil {
		return nil, 0, 0, false
	}
	if uint64(len(data)) != uint64(rec.Width)*uint64(rec.Height)*4 {
		return nil, 0, 0, false
	}
	return data, rec.Width, rec.Height, true
}

// yySkinCacheHas 只读元数据、不读像素：账号摘要每 10 秒就要问一遍每个账号，
// 用 yySkinCacheGet 会把整张纹理（几十 KB）读出来，纯属浪费。
func yySkinCacheHas(name string) (uint32, uint32, bool) {
	if yySkinDiskRoot == "" {
		return 0, 0, false
	}
	key := yySkinDiskKey(name)
	raw, err := os.ReadFile(filepath.Join(yySkinDiskRoot, key+".json"))
	if err != nil {
		return 0, 0, false
	}
	var rec yySkinDiskRecord
	if json.Unmarshal(raw, &rec) != nil || rec.Width == 0 || rec.Height == 0 {
		return 0, 0, false
	}
	if info, err := os.Stat(filepath.Join(yySkinDiskRoot, key+".rgba")); err != nil || info.Size() != int64(rec.Width)*int64(rec.Height)*4 {
		return 0, 0, false
	}
	return rec.Width, rec.Height, true
}

// yySkinCachedNames 返回磁盘里已有皮肤的玩家名（诊断用）。
func yySkinCachedNames() []string {
	if yySkinDiskRoot == "" {
		return nil
	}
	entries, err := os.ReadDir(yySkinDiskRoot)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(yySkinDiskRoot, entry.Name()))
		if err != nil {
			continue
		}
		var rec yySkinDiskRecord
		if json.Unmarshal(raw, &rec) == nil && rec.Name != "" {
			names = append(names, rec.Name)
		}
	}
	return names
}
