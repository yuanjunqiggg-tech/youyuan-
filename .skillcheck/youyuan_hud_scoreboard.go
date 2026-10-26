package main

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type yyHudEntry struct {
	Owner, Player, Channel, Kind, Text string
	Priority                           int
	Expires                            time.Time
	Seq                                uint64
}
type yyHudRendered struct {
	Seq    uint64
	Text   string
	SentAt time.Time
}

var yyHud = struct {
	sync.Mutex
	Items    map[*BotManager]map[string]yyHudEntry
	Rendered map[*BotManager]map[string]yyHudRendered
	Running  map[*BotManager]bool
	Wake     map[*BotManager]chan struct{}
	Seq      uint64
}{Items: map[*BotManager]map[string]yyHudEntry{}, Rendered: map[*BotManager]map[string]yyHudRendered{}, Running: map[*BotManager]bool{}, Wake: map[*BotManager]chan struct{}{}}

func init() {
	yyExtraMethods["hud.set"] = "player,channel,{text,kind=actionbar/title/subtitle,priority=0,duration_ms=2000}；宿主统一频道仲裁，优先级最高的活跃频道显示，文本变化更新，过期恢复下一频道。默认不重播动画。仅管理经hud接口发送的内容，无法阻止绕开管理器的原始titleraw。"
	yyExtraMethods["hud.update"] = yyExtraMethods["hud.set"]
	yyExtraMethods["hud.remove"] = "player,channel；删除本插件频道，下一调度恢复仍活跃频道；卸载插件自动清理。"
	yyExtraMethods["scoreboard.get"] = "objective,player；读取SetScore已同步条目，未广播known=false，不同步查询全服。"
	yyExtraMethods["scoreboard.set"] = "objective,player,value整数；提交计分板设置，submitted=true/confirmed=false，调用Confirmed命令或同步SetScore才能验证结果。"
	yyExtraMethods["scoreboard.increment"] = "objective,player,amount整数；提交原生scoreboard add/remove，不本地读改写，避免竞态；提交不等于服务器成功。"
}
func yyPlayerTarget(name string) (string, error) {
	if name == "self" || name == "@s" {
		return "@s", nil
	}
	if name == "" || len(name) > 256 || strings.ContainsAny(name, "\x00\r\n") {
		return "", fmt.Errorf("无效玩家名")
	}
	q, _ := json.Marshal(name)
	return "@a[name=" + string(q) + "]", nil
}
func yyHudKey(owner, player, channel string) string {
	return owner + "\x00" + player + "\x00" + channel
}
func yyHudCall(bm *BotManager, owner, method string, args []any) (any, error) {
	if bm == nil || !bm.IsConnected() {
		return nil, fmt.Errorf("游戏账号未连接")
	}
	player, e := yyArgString(args, 0, "")
	if e != nil {
		return nil, e
	}
	if _, e = yyPlayerTarget(player); e != nil {
		return nil, e
	}
	channel, e := yyArgString(args, 1, "")
	if e != nil || channel == "" || len(channel) > 128 {
		return nil, fmt.Errorf("需要channel字符串")
	}
	key := yyHudKey(owner, player, channel)
	var entry yyHudEntry
	if method != "hud.remove" {
		if len(args) < 3 {
			return nil, fmt.Errorf("需要显示配置")
		}
		cfg, ok := args[2].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("显示配置需对象")
		}
		text, ok := cfg["text"].(string)
		if !ok || len(text) > 4096 {
			return nil, fmt.Errorf("text需最多4096字节")
		}
		kind := "actionbar"
		if v, ok := cfg["kind"].(string); ok {
			kind = v
		}
		if kind != "actionbar" && kind != "title" && kind != "subtitle" {
			return nil, fmt.Errorf("kind无效")
		}
		priority := float64(0)
		if v, exists := cfg["priority"]; exists {
			var ok bool
			priority, ok = v.(float64)
			if !ok || math.IsNaN(priority) || math.IsInf(priority, 0) || math.Abs(priority) > 1000000 || priority != math.Trunc(priority) {
				return nil, fmt.Errorf("priority需整数±1000000")
			}
		}
		ms := float64(2000)
		if v, exists := cfg["duration_ms"]; exists {
			var ok bool
			ms, ok = v.(float64)
			if !ok || math.IsNaN(ms) || ms < 100 || ms > 86400000 {
				return nil, fmt.Errorf("duration_ms需100..86400000")
			}
		}
		entry = yyHudEntry{Owner: owner, Player: player, Channel: channel, Kind: kind, Text: text, Priority: int(priority), Expires: time.Now().Add(time.Duration(ms) * time.Millisecond)}
	}
	yyHud.Lock()
	defer yyHud.Unlock()
	if yyHud.Items[bm] == nil {
		yyHud.Items[bm] = map[string]yyHudEntry{}
	}
	if method == "hud.remove" {
		delete(yyHud.Items[bm], key)
	} else {
		if len(yyHud.Items[bm]) >= 1024 {
			if _, ok := yyHud.Items[bm][key]; !ok {
				return nil, fmt.Errorf("活跃HUD频道已满1024，请释放旧频道")
			}
		}
		yyHud.Seq++
		entry.Seq = yyHud.Seq
		yyHud.Items[bm][key] = entry
	}
	if !yyHud.Running[bm] {
		yyHud.Running[bm] = true
		yyHud.Wake[bm] = make(chan struct{}, 1)
		go yyHudLoop(bm)
	}
	select {
	case yyHud.Wake[bm] <- struct{}{}:
	default:
	}
	return map[string]any{"queued": true, "submitted": false, "confirmed": false, "channel": channel}, nil
}
func yyHudClearOwner(bm *BotManager, owner string) {
	yyHud.Lock()
	defer yyHud.Unlock()
	for k, e := range yyHud.Items[bm] {
		if e.Owner == owner {
			delete(yyHud.Items[bm], k)
		}
	}
	select {
	case yyHud.Wake[bm] <- struct{}{}:
	default:
	}
}
func yyHudWinners(items map[string]yyHudEntry, now time.Time) map[string]yyHudEntry {
	w := map[string]yyHudEntry{}
	for k, e := range items {
		if !e.Expires.After(now) {
			delete(items, k)
			continue
		}
		key := e.Player + "\x00" + e.Kind
		old, ok := w[key]
		if !ok || e.Priority > old.Priority || (e.Priority == old.Priority && e.Seq > old.Seq) {
			w[key] = e
		}
	}
	return w
}
func yyHudLoop(bm *BotManager) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	yyHud.Lock()
	wake := yyHud.Wake[bm]
	yyHud.Unlock()
	defer func() {
		yyHud.Lock()
		delete(yyHud.Items, bm)
		delete(yyHud.Rendered, bm)
		delete(yyHud.Running, bm)
		delete(yyHud.Wake, bm)
		yyHud.Unlock()
	}()
	for {
		select {
		case <-ticker.C:
		case <-wake:
		}
		if !bm.IsConnected() {
			return
		}
		yyHud.Lock()
		now := time.Now()
		winners := yyHudWinners(yyHud.Items[bm], now)
		old := yyHud.Rendered[bm]
		if old == nil {
			old = map[string]yyHudRendered{}
		}
		pending := []yyHudEntry{}
		for key, e := range winners {
			prev, ok := old[key]
			if !ok || prev.Text != e.Text || (e.Kind == "actionbar" && now.Sub(prev.SentAt) >= time.Second) {
				pending = append(pending, e)
				prev.SentAt = now
			}
			old[key] = yyHudRendered{e.Seq, e.Text, prev.SentAt}
		}
		for key := range old {
			if _, ok := winners[key]; !ok {
				parts := strings.Split(key, "\x00")
				pending = append(pending, yyHudEntry{Player: parts[0], Kind: parts[1], Text: ""})
				delete(old, key)
			}
		}
		yyHud.Rendered[bm] = old
		yyHud.Unlock()
		for _, e := range pending {
			target, _ := yyPlayerTarget(e.Player)
			if e.Kind == "title" {
				ticks := int(math.Ceil(time.Until(e.Expires).Seconds() * 20))
				if ticks < 1 {
					ticks = 1
				}
				_, _ = yyPluginCommandCall(bm, "game.sendCommandFast", []any{fmt.Sprintf("titleraw %s times 0 %d 0", target, ticks)}, yyDefaultCommandOptions())
			}
			raw, _ := json.Marshal(map[string]any{"rawtext": []any{map[string]any{"text": e.Text}}})
			_, err := yyPluginCommandCall(bm, "game.sendCommandFast", []any{fmt.Sprintf("titleraw %s %s %s", target, e.Kind, raw)}, yyDefaultCommandOptions())
			if err != nil {
				debugLog("[HUD] %s: %v", e.Channel, err)
			}
		}

	}
}

var yyScoreObjective = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

func yyScoreboardCall(bm *BotManager, method string, args []any) (any, error) {
	objective, e := yyArgString(args, 0, "")
	if e != nil || !yyScoreObjective.MatchString(objective) {
		return nil, fmt.Errorf("objective需1..64位字母数字_.-")
	}
	player, e := yyArgString(args, 1, "")
	if e != nil {
		return nil, e
	}
	selector, e := yyPlayerTarget(player)
	if e != nil {
		return nil, e
	}
	if method == "scoreboard.get" {
		s := yySenseFor(bm)
		s.Lock()
		defer s.Unlock()
		for _, row := range s.Scores {
			if row["objective"] == objective && row["player"] == player {
				return yyObservationCopy(row), nil
			}
		}
		return map[string]any{"known": false, "objective": objective, "player": player, "reason": "未收到该条目SetScore"}, nil
	}
	n, e := yyArgNumber(args, 2, 0)
	if e != nil || n != math.Trunc(n) || n < math.MinInt32 || n > math.MaxInt32 {
		return nil, fmt.Errorf("value需int32整数")
	}
	op := "set"
	if method == "scoreboard.increment" {
		op = "add"
		if n < 0 {
			op = "remove"
			n = -n
		}
	}
	return yyPluginCommandCall(bm, "game.sendCommandFast", []any{fmt.Sprintf("scoreboard players %s %s %s %s", op, selector, objective, strconv.FormatInt(int64(n), 10))}, yyDefaultCommandOptions())
}
