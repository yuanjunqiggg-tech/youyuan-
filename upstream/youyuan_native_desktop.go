//go:build !android

package main

import (
	"bufio"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

//go:embed sdk/native_worker.py
var yyNativeWorkerSource string

type yyNativeProcess struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	input  io.WriteCloser
	output *bufio.Reader
	closed bool
	id     uint64
	plugin string
}

var yyNativeProcesses = struct {
	sync.Mutex
	next  uint64
	items map[uint64]*yyNativeProcess
}{items: map[uint64]*yyNativeProcess{}}

func yyNativeStart(entry, account, pluginID string) (uint64, error) {
	if runtime.GOOS != "linux" {
		return 0, fmt.Errorf("当前原生库是 ELF；Windows 请使用 Linux 执行端或 Python/Lua，不能直接加载 Linux .so")
	}
	path, e := filepath.Abs(entry)
	if e != nil {
		return 0, e
	}
	root, e := filepath.Abs(yyRuntimeRoot)
	if e != nil {
		return 0, e
	}
	rel, e := filepath.Rel(root, path)
	if e != nil || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return 0, fmt.Errorf("原生入口必须位于插件目录")
	}
	if e = yyValidateNative(path); e != nil {
		return 0, e
	}
	python := os.Getenv("YY_PYTHON")
	if python == "" {
		python, e = exec.LookPath("python3")
	}
	if python == "" || e != nil {
		return 0, fmt.Errorf("Linux 原生隔离执行端需要 Python3 或 YY_PYTHON")
	}
	script := "import base64\nexec(base64.b64decode('" + base64.StdEncoding.EncodeToString([]byte(yyNativeWorkerSource)) + "'))"
	p := &yyNativeProcess{cmd: exec.Command(python, "-u", "-c", script), plugin: pluginID}
	p.cmd.Stderr = os.Stderr
	p.input, e = p.cmd.StdinPipe()
	if e != nil {
		return 0, e
	}
	output, e := p.cmd.StdoutPipe()
	if e != nil {
		p.input.Close()
		return 0, e
	}
	p.output = bufio.NewReader(output)
	if e = p.cmd.Start(); e != nil {
		p.input.Close()
		return 0, e
	}
	_, e = p.request(map[string]any{"action": "start", "entry": path, "account": account, "plugin_id": pluginID, "port": yyCorePort})
	if e != nil {
		return 0, e
	}
	yyNativeProcesses.Lock()
	yyNativeProcesses.next++
	p.id = yyNativeProcesses.next
	yyNativeProcesses.items[p.id] = p
	yyNativeProcesses.Unlock()
	return p.id, nil
}

// request serializes handlers within the resident native instance, and kills only
// its child process on timeout/crash. No per-command CPS limit is added here.
func (p *yyNativeProcess) request(value map[string]any) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return "", fmt.Errorf("原生插件 %s 的进程已停止", p.plugin)
	}
	data, e := json.Marshal(value)
	if e != nil {
		return "", fmt.Errorf("原生插件 %s 参数编码失败: %w", p.plugin, e)
	}
	type result struct {
		text string
		err  error
	}
	resultCh := make(chan result, 1)
	go func() {
		if _, e := p.input.Write(append(data, '\n')); e != nil {
			resultCh <- result{err: e}
			return
		}
		text, e := p.output.ReadString('\n')
		resultCh <- result{text, e}
	}()
	deadline := 120 * time.Second
	if configured, e := time.ParseDuration(os.Getenv("YY_NATIVE_TIMEOUT")); e == nil && configured > 0 {
		deadline = configured
	}
	timer := time.NewTimer(deadline)
	defer timer.Stop()
	var answer result
	select {
	case answer = <-resultCh:
	case <-timer.C:
		p.closeLocked()
		return "", fmt.Errorf("原生插件 %s handler=%v 超过 %s，仅终止该插件进程", p.plugin, value["action"], deadline)
	}
	if answer.err != nil {
		p.closeLocked()
		return "", fmt.Errorf("原生插件 %s 进程退出或协议失败: %w", p.plugin, answer.err)
	}
	var reply map[string]any
	if e = json.Unmarshal([]byte(answer.text), &reply); e != nil {
		p.closeLocked()
		return "", fmt.Errorf("原生插件 %s 返回无效 JSON: %w", p.plugin, e)
	}
	if reply["ok"] == false && value["action"] == "start" {
		p.closeLocked()
		return "", fmt.Errorf("原生插件 %s 初始化失败: %v", p.plugin, reply["error"])
	}
	return answer.text, nil
}
func (p *yyNativeProcess) closeLocked() {
	if p.closed {
		return
	}
	p.closed = true
	p.input.Close()
	_ = p.cmd.Process.Kill()
	_ = p.cmd.Wait()
	yyNativeProcesses.Lock()
	delete(yyNativeProcesses.items, p.id)
	yyNativeProcesses.Unlock()
}
func yyNativeEvent(id uint64, event string) (string, error) {
	yyNativeProcesses.Lock()
	p := yyNativeProcesses.items[id]
	yyNativeProcesses.Unlock()
	if p == nil {
		return "", fmt.Errorf("原生实例已停止")
	}
	return p.request(map[string]any{"action": "event", "event": event})
}
func yyNativeStop(id uint64) {
	yyNativeProcesses.Lock()
	p := yyNativeProcesses.items[id]
	yyNativeProcesses.Unlock()
	if p == nil {
		return
	}
	_, _ = p.request(map[string]any{"action": "stop"})
	p.mu.Lock()
	p.closeLocked()
	p.mu.Unlock()
}
