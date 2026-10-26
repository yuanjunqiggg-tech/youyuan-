package main

import (
	"strings"
	"testing"
	"time"
)

func TestAttachmentTextAndCharacterPaging(t *testing.T) {
	if got := yyAttachmentText([]byte{0xff, 0xfe, 0x2d, 0x4e, 0x87, 0x65}); got != "中文" {
		t.Fatal(got)
	}
	if got := yyAttachmentText([]byte{0xef, 0xbb, 0xbf, 'a'}); got != "a" {
		t.Fatal(got)
	}
	raw := string([]byte{'a', 0, 'b'})
	if yyAttachmentText([]byte(raw)) != raw {
		t.Fatal("binary altered")
	}
	text := strings.Repeat("文", 30000) + "文件尾标记"
	page := yyAgentAttachmentPage(text, 30000, 6000)
	if !strings.Contains(page, "文件尾标记") || !strings.Contains(page, "30000-30005") {
		t.Fatal(page)
	}
}

func TestConfigurableBurstBudget(t *testing.T) {
	if yyControlBatchPace(1024, 0) != 0 {
		t.Fatal("default must not add an artificial delay")
	}
	if yyControlBatchPace(1024, 2000) != 512*time.Millisecond {
		t.Fatal("custom pace incorrect")
	}
	for _, rate := range []int{-1, 1, 20001} {
		if yyChainValidate(yyChainConfig{Mode: "burst", RateCPS: rate}) == nil {
			t.Fatal("invalid rate", rate)
		}
	}
	options, err := yyMergeCommandOptions(yyDefaultCommandOptions(), map[string]any{"mode": "burst", "rate_cps": 2000})
	if err != nil || options.RateCPS != 2000 || options.Window != 1024 {
		t.Fatal(options, err)
	}
}
