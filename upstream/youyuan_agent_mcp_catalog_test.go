package main

import (
	"context"
	"strings"
	"testing"
)

func TestMCPToolCatalogLoadsOnlySelectedSchema(t *testing.T) {
	raw := []byte(`{"tools":[{"name":"read_file","description":"读取文件","inputSchema":{"type":"object","properties":{"path":{"type":"string"}}}},{"name":"run_build","description":"编译项目","inputSchema":{"type":"object","properties":{"target":{"type":"string"}}}}],"nextCursor":"page-two"}`)
	list := yyMCPToolDirectory(raw)
	if !strings.Contains(list, "read_file") || strings.Contains(list, "inputSchema") || !strings.Contains(list, "page-two") {
		t.Fatalf("目录应仅含简短说明及分页游标: %s", list)
	}
	doc, failed := yyMCPToolDescription(context.Background(), nil, "", "run_build", raw)
	if failed || !strings.Contains(doc, `"target"`) || strings.Contains(doc, `"read_file"`) {
		t.Fatalf("只应加载选中工具文档: %v %s", failed, doc)
	}
}
