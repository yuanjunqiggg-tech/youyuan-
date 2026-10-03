//go:build !android

package main

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

//go:embed android/app/src/main/python/youyuan_plugins.py
var yyPythonSource string

type yyPythonProcess struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	input  io.WriteCloser
	output *bufio.Reader
}

var yyPythonMu sync.Mutex
var yyPythonProcesses = map[string]*yyPythonProcess{}

func yyPythonValidate(content, path string) (string, error) {
	executable := os.Getenv("YY_PYTHON")
	if executable == "" {
		for _, name := range []string{"python3", "python"} {
			if path, e := exec.LookPath(name); e == nil {
				executable = path
				break
			}
		}
		if executable == "" {
			return "", fmt.Errorf("需安装Python3或设置YY_PYTHON")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	params, _ := json.Marshal(map[string]any{"action": "validate", "key": "validate", "content": content, "path": path})
	script := "import sys,base64\nexec(base64.b64decode('" + base64.StdEncoding.EncodeToString([]byte(yyPythonSource)) + "'))\nprint(dispatch(sys.stdin.read()))"
	command := exec.CommandContext(ctx, executable, "-c", script)
	command.Stdin = bytes.NewReader(params)
	data, e := command.Output()
	return string(data), e
}

func yyNativeStart(entry, account, pluginID string) (uint64, error) {
	return 0, fmt.Errorf("桌面原生插件适配器尚未接入；Android 内置适配器已实现")
}
func yyNativeEvent(id uint64, event string) (string, error) {
	return "", fmt.Errorf("桌面原生插件适配器尚未接入")
}
func yyNativeStop(id uint64) {}
func yyPythonRuntime(action string, params map[string]any) (string, error) {
	params["port"] = yyCorePort
	yyPythonMu.Lock()
	locked := true
	defer func() {
		if locked {
			yyPythonMu.Unlock()
		}
	}()
	key, _ := params["key"].(string)
	process := yyPythonProcesses[key]
	if action == "start" {
		if process != nil {
			return "", fmt.Errorf("Python 实例已经运行")
		}
		executable := os.Getenv("YY_PYTHON")
		if executable == "" {
			for _, name := range []string{"python3", "python"} {
				if path, err := exec.LookPath(name); err == nil {
					executable = path
					break
				}
			}
		}
		if executable == "" {
			return "", fmt.Errorf("桌面环境需安装 Python 3.12，或配置 YY_PYTHON")
		}
		script := "import base64,sys,contextlib\nexec(base64.b64decode('" + base64.StdEncoding.EncodeToString([]byte(yyPythonSource)) + "'))\nfor line in sys.stdin:\n try:\n  with contextlib.redirect_stdout(sys.stderr): result=dispatch(line)\n  print(result,flush=True)\n except Exception as error: print(json.dumps({'ok':False,'error':str(error)}),flush=True)\n"
		command := exec.Command(executable, "-u", "-c", script)
		command.Stderr = os.Stderr
		input, err := command.StdinPipe()
		if err != nil {
			return "", err
		}
		output, err := command.StdoutPipe()
		if err != nil {
			return "", err
		}
		if err = command.Start(); err != nil {
			return "", err
		}
		process = &yyPythonProcess{cmd: command, input: input, output: bufio.NewReader(output)}
		yyPythonProcesses[key] = process
	}
	if process == nil {
		if action == "stop" {
			return `{"ok":true}`, nil
		}
		return "", fmt.Errorf("Python 实例未运行")
	}
	yyPythonMu.Unlock()
	locked = false
	process.mu.Lock()
	defer process.mu.Unlock()
	params["action"] = action
	data, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	if _, err = process.input.Write(append(data, '\n')); err != nil {
		return "", err
	}
	result, err := process.output.ReadString('\n')
	cleanup := action == "stop" || err != nil
	if action == "start" && err == nil {
		var answer map[string]any
		_ = json.Unmarshal([]byte(result), &answer)
		cleanup = answer["ok"] == false
	}
	if cleanup {
		yyPythonMu.Lock()
		delete(yyPythonProcesses, key)
		yyPythonMu.Unlock()
		process.input.Close()
		_ = process.cmd.Process.Kill()
		_ = process.cmd.Wait()
	}
	return result, err
}
