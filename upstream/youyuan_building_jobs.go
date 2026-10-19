package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
)

var yyBuildingJobMethods = map[string]string{
	"building.taskList":     "无参数；列出当前账号/当前服务器的真实作业断点、原参数、进度和状态。running不等于服务器已确认所有方块。",
	"building.startImport":  "{path,x,y,z,speed,dimension?,rotation?,multi?,bot_names?}；调用宿主建筑导入，不经过插件自建导入循环。multi使用同服已就绪且明确选中的机器人。返回已提交；用taskList查进度。",
	"building.resumeImport": "task_id；按保留的原坐标、文件校验和分区布局恢复。文件改变或机器人不就绪明确报错。",
	"building.stopImport":   "无参数；请求停止当前账号作业，等待清理和断点落盘期间禁止启动重叠作业。",
}

func init() {
	for method, doc := range yyBuildingJobMethods {
		yyExtraMethods[method] = doc
	}
}

func yyBuildingJobCall(bm *BotManager, method string, args []any) (any, error) {
	if current := getBotManager(); bm != nil && current != bm {
		return nil, fmt.Errorf("建筑任务绑定当前账号内核，请通过目标账号工作空间调用")
	}
	var handler http.HandlerFunc
	verb, path := "POST", ""
	var body any
	switch method {
	case "building.taskList":
		handler = handleTaskList
		verb = "GET"
		path = "/api/task/list"
	case "building.startImport":
		if len(args) < 1 {
			return nil, fmt.Errorf("需要导入参数对象")
		}
		raw, e := json.Marshal(args[0])
		if e != nil {
			return nil, e
		}
		var params map[string]any
		if e = json.Unmarshal(raw, &params); e != nil || params == nil {
			return nil, fmt.Errorf("导入参数必须是对象")
		}
		kind := "import"
		if multi, _ := params["multi"].(bool); multi {
			kind = "import_multi"
		}
		delete(params, "multi")
		body = map[string]any{"type": kind, "params": params}
		handler = handleTaskStart
		path = "/api/task/start"
	case "building.resumeImport":
		if len(args) < 1 {
			return nil, fmt.Errorf("需要task_id")
		}
		id, ok := args[0].(string)
		if !ok || id == "" {
			return nil, fmt.Errorf("task_id必须是非空字符串")
		}
		body = map[string]any{"task_id": id}
		handler = handleTaskResume
		path = "/api/task/resume"
	case "building.stopImport":
		handler = handleTaskStop
		path = "/api/task/stop"
	default:
		return nil, fmt.Errorf("未知建筑作业接口")
	}
	raw, e := json.Marshal(body)
	if e != nil {
		return nil, e
	}
	response := httptest.NewRecorder()
	handler(response, httptest.NewRequest(verb, path, bytes.NewReader(raw)))
	var result map[string]any
	if e = json.Unmarshal(response.Body.Bytes(), &result); e != nil {
		return nil, e
	}
	if ok, _ := result["ok"].(bool); !ok {
		return nil, fmt.Errorf("%v", result["error"])
	}
	if method == "building.startImport" || method == "building.resumeImport" {
		result["submitted"] = true
		result["completed"] = false
	}
	return result, nil
}
