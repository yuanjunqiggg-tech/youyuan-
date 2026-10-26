package main

import (
	"context"
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

type yyMCPToolInfo struct {
	Name         string          `json:"name"`
	Title        string          `json:"title,omitempty"`
	Description  string          `json:"description,omitempty"`
	InputSchema  json.RawMessage `json:"inputSchema,omitempty"`
	OutputSchema json.RawMessage `json:"outputSchema,omitempty"`
}

type yyMCPToolPage struct {
	Tools      []yyMCPToolInfo `json:"tools"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

func yyMCPToolDirectory(data []byte) string {
	var page yyMCPToolPage
	if json.Unmarshal(data, &page) != nil {
		return string(data)
	}
	items := make([]map[string]string, 0, len(page.Tools))
	for _, tool := range page.Tools {
		description := tool.Description
		if utf8.RuneCountInString(description) > 160 {
			description = string([]rune(description)[:160]) + "…"
		}
		items = append(items, map[string]string{"name": tool.Name, "title": tool.Title, "summary": description})
	}
	return mustJSON(map[string]any{"tools": items, "nextCursor": page.NextCursor})
}

func yyMCPToolDescription(ctx context.Context, ws *yyWorkspace, serverID, name string, firstPage []byte) (string, bool) {
	if name == "" {
		return "请指定工具名", true
	}
	data := firstPage
	for pageNumber := 0; pageNumber < 20; pageNumber++ {
		var page yyMCPToolPage
		if err := json.Unmarshal(data, &page); err != nil {
			return fmt.Sprintf("外部 MCP 工具目录无效: %v", err), true
		}
		for _, tool := range page.Tools {
			if tool.Name == name {
				return mustJSON(tool), false
			}
		}
		if page.NextCursor == "" {
			return "外部 MCP 未提供工具 " + name, true
		}
		var err error
		data, err = yyWorkspaceMCP(ctx, ws, serverID, "tools/list", map[string]any{"cursor": page.NextCursor})
		if err != nil {
			return err.Error(), true
		}
	}
	return "外部 MCP 工具目录超过 20 页，请先缩小范围", true
}
