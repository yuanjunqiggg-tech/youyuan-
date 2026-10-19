package main

import (
	"context"
	"fmt"
	lua "github.com/yuin/gopher-lua"
	"time"
)

type yyLuaWaitKey struct{}

// Only Go I/O can run outside the VM lock. All Lua inputs must be copied before
// calling this function; no Lua values or stack operations occur in operation.
func yyLuaWait(L *lua.LState, operation func(context.Context) (string, error)) (string, error) {
	return yyLuaWaitValue(L, operation)
}

func yyLuaWaitValue[T any](L *lua.LState, operation func(context.Context) (T, error)) (T, error) {
	ctx := L.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	r, _ := ctx.Value(yyLuaWaitKey{}).(*yyLuaResident)
	if r == nil {
		return operation(ctx)
	} // initialization/SDK keeps its own lock
	r.ioWaiting.Add(1)
	started := time.Now()
	r.mu.Unlock()
	var value T
	var err error
	func() {
		defer r.mu.Lock()
		defer func() {
			r.ioWaiting.Add(-1)
			r.ioCalls.Add(1)
			ns := time.Since(started).Nanoseconds()
			r.ioTotalNS.Add(ns)
			for old := r.ioMaxNS.Load(); ns > old && !r.ioMaxNS.CompareAndSwap(old, ns); old = r.ioMaxNS.Load() {
			}
		}()
		value, err = operation(ctx)
	}()
	if r.closed || ctx.Err() != nil {
		var zero T
		return zero, fmt.Errorf("插件 I/O 已取消: %v", ctx.Err())
	}
	return value, err
}

// Bound Go methods and copied arguments only; never access the Lua stack while
// suspended. Preserve the old synchronous result while other callbacks run.
func yyLuaWait1[A, T any](L *lua.LState, operation func(A) (T, error), argument A) (T, error) {
	return yyLuaWaitValue(L, func(_ context.Context) (T, error) { return operation(argument) })
}
