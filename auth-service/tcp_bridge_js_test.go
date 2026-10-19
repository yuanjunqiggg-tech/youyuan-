//go:build js && wasm

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"syscall/js"
	"testing"
	"time"
)

func workersFactory(body string) js.Value { return js.Global().Get("Function").New("address", body) }
func TestWorkersBridgeBytesAndEOF(t *testing.T) {
	f := workersFactory(`let sent=false,closed=false;return {opened:Promise.resolve({}),read:async n=>{if(sent)return new Uint8Array();sent=true;return Uint8Array.of(4,5);},write:async b=>{if(b[0]!==9)throw Error('wrong payload');return b.length;},close(){closed=true;}}`)
	c, e := dialWorkersTCP(context.Background(), f, "tcp", "223.252.200.165:10201")
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	b := make([]byte, 4)
	if n, e := c.Read(b); e != nil || n != 2 || b[0] != 4 || b[1] != 5 {
		t.Fatal(n, e, b)
	}
	if n, e := c.Write([]byte{9, 8}); e != nil || n != 2 {
		t.Fatal(n, e)
	}
	if n, e := c.Read(b); n != 0 || !errors.Is(e, io.EOF) {
		t.Fatal(n, e)
	}
}
func TestWorkersBridgeCancelledOpenAndReadDeadline(t *testing.T) {
	f := workersFactory(`let reject;return {opened:new Promise((r,j)=>reject=j),close(){reject(Error('closed'));}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	if _, e := dialWorkersTCP(ctx, f, "tcp", "1.2.3.4:10200"); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
	f = workersFactory(`let resolve;return {opened:Promise.resolve({}),read:()=>new Promise(r=>resolve=r),close(){resolve?.(new Uint8Array());}}`)
	c, e := dialWorkersTCP(context.Background(), f, "tcp", "1.2.3.4:10200")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { _, e := c.Read(make([]byte, 4)); done <- e }()
	time.Sleep(time.Millisecond)
	c.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	select {
	case e := <-done:
		if !errors.Is(e, os.ErrDeadlineExceeded) {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("pending read ignored deadline update")
	}
}
func TestWorkersBridgeCloseInterruptsWrite(t *testing.T) {
	f := workersFactory(`let reject;return {opened:Promise.resolve({}),write:()=>new Promise((r,j)=>reject=j),close(){reject?.(Error('closed'));}}`)
	c, e := dialWorkersTCP(context.Background(), f, "tcp", "1.2.3.4:10201")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { _, e := c.Write([]byte{1}); done <- e }()
	time.Sleep(time.Millisecond)
	c.Close()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("closed write succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("close did not unblock write")
	}
}
