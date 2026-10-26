package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type yyChatRecord struct {
	Time    string `json:"time"`
	Player  string `json:"player"`
	Message string `json:"message"`
	Server  string `json:"server"`
}
type yyOnlineSample struct {
	Time      string `json:"time"`
	Online    *int   `json:"online"`
	Server    string `json:"server"`
	Connected bool   `json:"connected"`
}

var yyObservation = struct {
	sync.Mutex
	Chat    []yyChatRecord
	Samples []yyOnlineSample
	Path    string
	Started bool
}{Chat: []yyChatRecord{}, Samples: []yyOnlineSample{}}
var yyChatQueue = make(chan yyChatRecord, 2048)

func yyRecordChat(bm *BotManager, player, message string) {
	bm.mu.Lock()
	server := bm.serverCode
	bm.mu.Unlock()
	record := yyChatRecord{Time: time.Now().Format(time.RFC3339Nano), Player: player, Message: message, Server: server}
	select {
	case yyChatQueue <- record:
	default:
		yyEvent("observation_error", "聊天记录队列已满，部分消息未入库")
	}
}
func yyOnlineSnapshot() (*int, string, bool) {
	bm := getBotManagerLocked()
	if bm == nil || !bm.IsConnected() {
		return nil, "", false
	}
	bm.mu.Lock()
	server := bm.serverCode
	bm.mu.Unlock()
	count := len(playerInfoMgr.OnlineSet())
	return &count, server, true
}
func yyObservationRoutes(mux *http.ServeMux, dataDir string) {
	yyObservation.Path = filepath.Join(dataDir, "account-observations.json")
	if data, e := os.ReadFile(yyObservation.Path); e == nil {
		var saved struct {
			Chat    []yyChatRecord   `json:"chat"`
			Samples []yyOnlineSample `json:"samples"`
		}
		if json.Unmarshal(data, &saved) == nil {
			yyObservation.Chat = saved.Chat
			yyObservation.Samples = saved.Samples
		}
	}
	mux.HandleFunc("/api/youyuan/observations", func(w http.ResponseWriter, r *http.Request) {
		online, server, connected := yyOnlineSnapshot()
		yyObservation.Lock()
		defer yyObservation.Unlock()
		writeJSON(w, map[string]any{"ok": true, "connected": connected, "online": online, "server": server, "source": "服务器发送的 PlayerList；只计内核实际收到的在线玩家，可能与服务端全量人数不同", "chat": yyObservation.Chat, "samples": yyObservation.Samples})
	})
	mux.HandleFunc("/api/youyuan/observations/export", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, map[string]any{"ok": false, "error": "使用 POST 导出记录"})
			return
		}
		yyObservation.Lock()
		chat := append([]yyChatRecord(nil), yyObservation.Chat...)
		samples := append([]yyOnlineSample(nil), yyObservation.Samples...)
		yyObservation.Unlock()
		data, err := json.MarshalIndent(map[string]any{"format": "youyuan.observations", "version": 1, "exported_at": time.Now().Format(time.RFC3339Nano), "chat": chat, "samples": samples}, "", "  ")
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		dir := filepath.Join(dataDir, "exports")
		if err = os.MkdirAll(dir, 0700); err == nil {
			err = os.WriteFile(filepath.Join(dir, "account-observations.json"), data, 0600)
		}
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "src_path": filepath.Join(dir, "account-observations.json"), "name": "account-observations.json", "chat_count": len(chat), "sample_count": len(samples)})
	})
	yyObservation.Started = true
	go func() {
		sample := time.NewTicker(15 * time.Second)
		save := time.NewTicker(5 * time.Second)
		defer sample.Stop()
		defer save.Stop()
		dirty := false
		for {
			select {
			case record := <-yyChatQueue:
				yyObservation.Lock()
				yyObservation.Chat = append(yyObservation.Chat, record)
				if len(yyObservation.Chat) > 10000 {
					yyObservation.Chat = yyObservation.Chat[len(yyObservation.Chat)-10000:]
				}
				yyObservation.Unlock()
				dirty = true
			case now := <-sample.C:
				online, server, connected := yyOnlineSnapshot()
				yyObservation.Lock()
				yyObservation.Samples = append(yyObservation.Samples, yyOnlineSample{Time: now.Format(time.RFC3339Nano), Online: online, Server: server, Connected: connected})
				if len(yyObservation.Samples) > 5760 {
					yyObservation.Samples = yyObservation.Samples[len(yyObservation.Samples)-5760:]
				}
				yyObservation.Unlock()
				dirty = true
			case <-save.C:
				if dirty {
					yyObservation.Lock()
					data, _ := json.Marshal(map[string]any{"chat": yyObservation.Chat, "samples": yyObservation.Samples})
					path := yyObservation.Path
					yyObservation.Unlock()
					if os.WriteFile(path+".tmp", data, 0600) == nil {
						_ = os.Rename(path+".tmp", path)
					}
					dirty = false
				}
			}
		}
	}()
}
