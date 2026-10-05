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
	"game.sendCommandAsync": "command,options?；立刻返回ticket，不阻塞插件回调；后台等待真实CommandOutput，使用game.commandResult读取。最多128个未读取ticket/实例，满时明确报错；Fast路径不走该窗口。",
	"game.commandResult":    "ticket,wait_ms=0,consume=false；pending=true表示结果未到。completed=true附result/error；只可读本实例ticket，consume=true删除完成记录。超时不重发。",
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
	if _, e := yySDKString(args, 0); e != nil {
		return nil, e
	}
	if len(args) > 1 {
		if _, e := yyMergeCommandOptions(options, args[1]); e != nil {
			return nil, e
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
	if count >= 128 {
		yyCommandTickets.Unlock()
		return nil, fmt.Errorf("插件未读取异步命令ticket已满(128)；请读取并consume，不影响Fast发送")
	}
	id := yyNextCommandTicket.Add(1)
	t := &yyAsyncCommand{Owner: owner, Done: make(chan struct{}), At: time.Now()}
	yyCommandTickets.Items[id] = t
	yyCommandTickets.Unlock()
	go func() { t.Value, t.Err = yyPluginCommandCall(owner.Bot, "game.execute", args, options); close(t.Done) }()
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
