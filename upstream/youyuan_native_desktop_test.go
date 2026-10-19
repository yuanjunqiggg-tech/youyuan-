//go:build !android

package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Separate process simulates a crashing/stalled library without crashing tests.
func TestNativeWorkerTransportHelper(t *testing.T) {
	mode := os.Getenv("YY_TEST_NATIVE_HELPER")
	if mode == "" {
		return
	}
	reader := bufio.NewScanner(os.Stdin)
	for reader.Scan() {
		if mode == "crash" {
			os.Exit(42)
		}
		if mode == "stall" {
			time.Sleep(time.Second)
		}
		value := map[string]any{"ok": true, "result": "resident"}
		if mode == "error" {
			value = map[string]any{"ok": false, "error": "plugin business error"}
		}
		_ = json.NewEncoder(os.Stdout).Encode(value)
	}
	os.Exit(0)
}
func nativeTransportFixture(t *testing.T, mode string) *yyNativeProcess {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestNativeWorkerTransportHelper$")
	command.Env = append(os.Environ(), "YY_TEST_NATIVE_HELPER="+mode)
	input, e := command.StdinPipe()
	if e != nil {
		t.Fatal(e)
	}
	output, e := command.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	if e = command.Start(); e != nil {
		t.Fatal(e)
	}
	p := &yyNativeProcess{cmd: command, input: input, output: bufio.NewReader(output), plugin: "test_" + mode}
	t.Cleanup(func() { p.mu.Lock(); p.closeLocked(); p.mu.Unlock() })
	return p
}
func TestNativeWorkerTransportCrashAndTimeout(t *testing.T) {
	crash := nativeTransportFixture(t, "crash")
	if _, e := crash.request(map[string]any{"action": "event"}); e == nil || !crash.closed {
		t.Fatal("worker crash was not contained", e)
	}
	t.Setenv("YY_NATIVE_TIMEOUT", "25ms")
	stall := nativeTransportFixture(t, "stall")
	if _, e := stall.request(map[string]any{"action": "event"}); e == nil || !stall.closed {
		t.Fatal("worker timeout was not contained", e)
	}
	t.Setenv("YY_NATIVE_TIMEOUT", "2s")
	good := nativeTransportFixture(t, "ok")
	for i := 0; i < 2; i++ {
		if _, e := good.request(map[string]any{"action": "event"}); e != nil || good.closed {
			t.Fatal("healthy resident worker affected by another worker", e)
		}
	}
}
func TestNativeWorkerBusinessErrorKeepsResident(t *testing.T) {
	p := nativeTransportFixture(t, "error")
	for i := 0; i < 2; i++ {
		text, e := p.request(map[string]any{"action": "event"})
		var reply map[string]any
		_ = json.Unmarshal([]byte(text), &reply)
		if e != nil || p.closed || reply["ok"] != false {
			t.Fatal(text, e)
		}
	}
}
