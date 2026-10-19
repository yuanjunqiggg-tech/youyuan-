package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type yyAsyncCommandKey struct {
	Bot    *BotManager
	Plugin string
}
type yyAsyncCommand struct {
	Owner yyAsyncCommandKey
	Done  chan struct{}
	Value any
	Err   error
	At    time.Time
}

var yyCommandTickets = struct {
	sync.Mutex
	Items map[uint64]*yyAsyncCommand
}{Items: map[uint64]*yyAsyncCommand{}}
var yyNextCommandTicket atomic.Uint64
var yyAsyncDocs = map[string]string{
	"game.sendCommandAsync":  "command,options?；立刻返回ticket，不阻塞插件回调；后台等待真实CommandOutput，使用game.commandResult读取。未读取ticket容量由diagnostics.configure的async_tickets控制，默认128，满时明确报错；Fast路径不走该窗口。",
	"game.commandResult":     "ticket,wait_ms=0,consume=false；pending=true表示结果未到。completed=true附result/error；只可读本实例ticket，consume=true删除完成记录。超时不重发。",
	"game.getBlockTileAsync": "x,y,z整数；后台执行原getBlockTileRaw的WS方块查询，不传送；立即返回ticket。game.commandResult(ticket,0,true)完成时result={block,confirmed=true}，失败返回error，不编造方块。适合事件处理器，不能用于查询未加载的全世界。",
}

func init() {
	for m, d := range yyAsyncDocs {
		yyExtraMethods[m] = d
	}
}

func yyAsyncCommandCall(owner yyAsyncCommandKey, method string, args []any, options yyCommandOptions) (any, error) {
	if method == "game.commandResult" {
		if len(args) < 1 {
			return nil, fmt.Errorf("缺少ticket")
		}
		n, ok := args[0].(float64)
		if !ok || n < 1 || n != float64(uint64(n)) {
			return nil, fmt.Errorf("ticket需正整数")
		}
		id := uint64(n)
		yyCommandTickets.Lock()
		t := yyCommandTickets.Items[id]
		yyCommandTickets.Unlock()
		if t == nil || t.Owner != owner {
			return nil, fmt.Errorf("ticket不属于当前插件或已释放")
		}
		ms := float64(0)
		if len(args) > 1 {
			ms, ok = args[1].(float64)
			if !ok || ms < 0 || ms > 10000 {
				return nil, fmt.Errorf("wait_ms需0..10000")
			}
		}
		if ms > 0 {
			timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
			select {
			case <-t.Done:
			case <-timer.C:
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}
		select {
		case <-t.Done:
			result := map[string]any{"ticket": id, "completed": true, "pending": false, "result": t.Value}
			if t.Err != nil {
				result["error"] = t.Err.Error()
			}
			if len(args) > 2 && args[2] == true {
				yyCommandTickets.Lock()
				delete(yyCommandTickets.Items, id)
				yyCommandTickets.Unlock()
			}
			return result, nil
		default:
			return map[string]any{"ticket": id, "completed": false, "pending": true}, nil
		}
	}
	if owner.Plugin == "" {
		return nil, fmt.Errorf("异步命令需要插件上下文")
	}
	// Validate before reserving an asynchronous request. Confirmed commands keep
	// their output association; ordinary fast sends never enter this registry.
	var xyz [3]int
	blockQuery := method == "game.getBlockTileAsync"
	if blockQuery {
		if owner.Bot == nil || !owner.Bot.IsConnected() {
			return nil, fmt.Errorf("游戏账号未连接")
		}
		for i := range xyz {
			n, e := yyArgNumber(args, i, 0)
			if len(args) <= i || e != nil || n != float64(int(n)) {
				return nil, fmt.Errorf("x/y/z需有限整数")
			}
			xyz[i] = int(n)
		}
	} else if _, e := yySDKString(args, 0); e != nil {
		return nil, e
	}
	if !blockQuery && len(args) > 1 {
		if _, e := yyMergeCommandOptions(options, args[1]); e != nil {
			return nil, e
		}
	}
	if !blockQuery {
		command, _ := yySDKString(args, 0)
		if busy, _ := yySpatialJobState(owner.Bot); busy && yyPluginTeleportCommand(owner.Bot, command) {
			return nil, yyPluginMovementAllowed(owner.Bot)
		}
	}
	yyCommandTickets.Lock()
	count := 0
	for id, t := range yyCommandTickets.Items {
		select {
		case <-t.Done:
			if time.Since(t.At) > time.Minute {
				delete(yyCommandTickets.Items, id)
			}
		default:
		}
		if t.Owner == owner {
			count++
		}
	}
	limit := yyObservationSettings().AsyncTickets
	if count >= limit {
		yyCommandTickets.Unlock()
		return nil, fmt.Errorf("插件未读取异步命令ticket已满(%d)；请读取并consume，可用diagnostics.configure设置async_tickets，不影响Fast发送", limit)
	}
	id := yyNextCommandTicket.Add(1)
	t := &yyAsyncCommand{Owner: owner, Done: make(chan struct{}), At: time.Now()}
	yyCommandTickets.Items[id] = t
	yyCommandTickets.Unlock()
	go func() {
		if blockQuery {
			var block string
			block, t.Err = owner.Bot.GetBlockTile(xyz[0], xyz[1], xyz[2])
			if t.Err == nil {
				t.Value = map[string]any{"block": block, "confirmed": true}
			}
		} else {
			t.Value, t.Err = yyPluginCommandCall(owner.Bot, "game.execute", args, options)
		}
		close(t.Done)
	}()
	return map[string]any{"ticket": id, "pending": true, "confirmed": false}, nil
}

func yyClearCommandTickets(owner yyAsyncCommandKey) {
	yyCommandTickets.Lock()
	defer yyCommandTickets.Unlock()
	for id, t := range yyCommandTickets.Items {
		if t.Owner == owner {
			delete(yyCommandTickets.Items, id)
		}
	}
}
