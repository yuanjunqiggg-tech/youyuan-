//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"
	"time"
)

func main() {
	done := make(chan struct{})
	callback := js.FuncOf(func(this js.Value, args []js.Value) any {
		raw := args[0].String()
		executor := js.FuncOf(func(this js.Value, p []js.Value) any {
			resolve := p[0]
			go func() {
				data, _ := json.Marshal(kernel(raw))
				resolve.Invoke(string(data))
				time.Sleep(time.Millisecond)
				close(done)
			}()
			return nil
		})
		promise := js.Global().Get("Promise").New(executor)
		executor.Release()
		return promise
	})
	js.Global().Set("youyuanAuthKernel", callback)
	<-done
	callback.Release()
}
