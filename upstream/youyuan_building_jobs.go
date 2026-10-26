package main

import (
	"bot-apk/building"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
)

var yyBuildingJobMethods = map[string]string{
	"building.importLimits": "{x?,z?}；返回常加载官方上限和当前导入器实际粒度；不执行游戏命令，不代表当前服务器已验收。",
	"building.taskList":     "无参数；列出当前账号/当前服务器的真实作业断点、原参数、进度和状态。running不等于服务器已确认所有方块。",
	"building.startImport":  "{path,x,y,z,speed,dimension?,rotation?,region_size_chunks?,preload_size_chunks?,ticking_preload?,multi?,bot_names?}；region_size_chunks=0沿用旧档位，1..10为单机每批边长；preload_size_chunks=0沿用批次，或选择批次整数倍的大窗口复用一个常加载区。10需要X/Z对齐16。multi当前按单区块单元分工。返回提交状态；用taskList查进度。",
	"building.resumeImport": "task_id；按保留的原坐标、文件校验和分区布局恢复。文件改变或机器人不就绪明确报错。",
	"building.stopImport":   "无参数；请求停止当前账号作业，等待清理和断点落盘期间禁止启动重叠作业。",
}

func init() {
	for method, doc := range yyBuildingJobMethods {
		yyExtraMethods[method] = doc
	}
}

func yyValidateImportRegionParams(kind string, p map[string]any) error {
	if kind != "import" && kind != "import_multi" {
		return nil
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	var v struct {
		Size    int `json:"region_size_chunks"`
		Preload int `json:"preload_size_chunks"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("导入批次与预加载窗口边长必须为整数: %w", err)
	}
	if kind == "import_multi" && v.Size > 1 {
		return fmt.Errorf("多机器人当前按 1×1 单元分工，暂不支持扩大单个单元；请省略 region_size_chunks")
	}
	if kind == "import_multi" && v.Preload > 1 {
		return fmt.Errorf("多机大范围预加载窗口尚未接入单元调度，请省略 preload_size_chunks")
	}
	x, z, enabled := getInt(p, "x", 0), getInt(p, "z", 0), getBool(p, "ticking_preload", true)
	if err := building.ValidateImportRegion(v.Size, x, z, enabled); err != nil {
		return err
	}
	batch := building.ImportRegionSize(getInt(p, "region_mode", 1), v.Size)
	if kind == "import_multi" {
		batch = 1
	}
	return building.ValidateImportPreload(v.Preload, batch, x, z, enabled)
}

func yyBuildingJobCall(bm *BotManager, method string, args []any) (any, error) {
	if current := getBotManager(); bm != nil && current != bm {
		return nil, fmt.Errorf("建筑任务绑定当前账号内核，请通过目标账号工作空间调用")
	}
	var handler http.HandlerFunc
	verb, path := "POST", ""
	var body any
	switch method {
	case "building.importLimits":
		var p map[string]any
		if len(args) > 0 {
			raw, err := json.Marshal(args[0])
			if err != nil {
				return nil, err
			}
			if err = json.Unmarshal(raw, &p); err != nil || p == nil {
				return nil, fmt.Errorf("参数必须为对象")
			}
		}
		side := building.MaxImportRegionChunks
		if getInt(p, "x", 0)%16 != 0 || getInt(p, "z", 0)%16 != 0 {
			side--
		}
		return map[string]any{"ok": true, "max_chunks_per_ticking_area": building.MaxTickingAreaChunks, "max_ticking_areas_per_world": building.MaxWorldTickingAreas, "max_region_size_chunks": side, "multi_unit_size_chunks": 1, "world_quota_shared": true, "server_verified": false}, nil
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
