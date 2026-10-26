package main

import "testing"

func TestPublicURLsKeepDeviceButNeverCredentials(t *testing.T) {
	urls := yyPublicURLs("wss://toolbox.youyuanqi.dpdns.org/device/connect?device=second-device&token=private")
	if urls["mcp"] != "https://toolbox.youyuanqi.dpdns.org/mcp/?device=second-device" {
		t.Fatal(urls)
	}
	if urls["console"] != "https://toolbox.youyuanqi.dpdns.org/?device=second-device" {
		t.Fatal(urls)
	}
	for _, raw := range []string{"ws://host/device/connect", "wss://user:password@host/device/connect", ":bad"} {
		if len(yyPublicURLs(raw)) != 0 {
			t.Fatal("unsafe URL accepted")
		}
	}
	if yyQQGroupString(nil) != "" {
		t.Fatal("unset official group must stay empty for callback binding")
	}
}
