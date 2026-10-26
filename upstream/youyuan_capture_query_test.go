package main

import (
	"encoding/json"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"testing"
)

func TestCaptureQueryDoesNotTreatEmptyExecuteSuccessAsDimension(t *testing.T) {
	p := &packet.CommandOutput{SuccessCount: 1, DataSet: `{"statusCode":0}`}
	if len(yyCaptureQueryRows(p)) != 0 {
		t.Fatal("empty execute output became known dimension")
	}
	raw := `[{"dimension":1,"id":-123,"uniqueId":"00000000-0000-4000-8000-00006c02bd9e"}]`
	p.OutputMessages = []protocol.CommandOutputMessage{{Parameters: []string{raw}}}
	rows := yyCaptureQueryRows(p)
	if len(rows) != 1 || rows[0].Dimension != 1 {
		t.Fatal(rows)
	}
	p.OutputMessages = nil
	dataset, _ := json.Marshal(map[string]any{"statusCode": 0, "details": raw})
	p.DataSet = string(dataset)
	rows = yyCaptureQueryRows(p)
	if len(rows) != 1 || rows[0].Dimension != 1 {
		t.Fatal("dataset fallback", rows)
	}
	// 2026-10-26 第 27 条：维度上限从 2 放开到 31（网易自定义维度 dm3/dm4…），
	// 所以 dimension=3 现在是**合法**的，不能再拿它当非法样本。
	p.DataSet = `{"details":[{"id":12},{"dimension":3,"id":13}]}`
	if rows := yyCaptureQueryRows(p); len(rows) != 1 || rows[0].Dimension != 3 {
		t.Fatalf("dimension=3（dm3）应被接受，实际 %v", rows)
	}
	// 真正非法的：缺 dimension 字段，以及超出上限 31 的值
	p.DataSet = `{"details":[{"id":12},{"dimension":40,"id":13}]}`
	if len(yyCaptureQueryRows(p)) != 0 {
		t.Fatal("missing/invalid dimension accepted")
	}
	p.SuccessCount = 0
	p.DataSet = string(dataset)
	if len(yyCaptureQueryRows(p)) != 0 {
		t.Fatal("rejected result accepted")
	}
}
