package main

import (
	"reflect"
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func TestAdmissionAcceptsOnlySuccessfulNonemptyCompleteList(t *testing.T) {
	valid := func() *packet.CommandOutput {
		return &packet.CommandOutput{SuccessCount: 1, OutputMessages: []protocol.CommandOutputMessage{{Success: true, Message: "commands.players.list", Parameters: []string{"3", "50"}}, {Success: true, Message: "commands.players.list.names", Parameters: []string{"observer, human-a, human-b"}}}}
	}
	names, ok := yyAdmissionListNames(valid())
	if !ok || !reflect.DeepEqual(names, []string{"observer", "human-a", "human-b"}) {
		t.Fatal(names, ok)
	}
	cases := map[string]func() *packet.CommandOutput{
		"nil": func() *packet.CommandOutput { return nil },
		"unknown execute success": func() *packet.CommandOutput {
			return &packet.CommandOutput{SuccessCount: 1, DataSet: `{"statusCode":0}`}
		},
		"failure":                  func() *packet.CommandOutput { p := valid(); p.SuccessCount = 0; return p },
		"message failure":          func() *packet.CommandOutput { p := valid(); p.OutputMessages[1].Success = false; return p },
		"empty":                    func() *packet.CommandOutput { p := valid(); p.OutputMessages[1].Parameters[0] = ""; return p },
		"truncated count mismatch": func() *packet.CommandOutput { p := valid(); p.OutputMessages[0].Parameters[0] = "23"; return p },
		"unknown count":            func() *packet.CommandOutput { p := valid(); p.OutputMessages[0].Parameters[0] = "unknown"; return p },
		"duplicates": func() *packet.CommandOutput {
			p := valid()
			p.OutputMessages[1].Parameters[0] = "observer, human-a, human-a"
			return p
		},
		"trailing empty": func() *packet.CommandOutput {
			p := valid()
			p.OutputMessages[1].Parameters[0] = "observer, human-a,"
			return p
		},
		"line fragment": func() *packet.CommandOutput {
			p := valid()
			p.OutputMessages[1].Parameters[0] = "observer, human-a\nhuman-b, human-c"
			return p
		},
	}
	for name, makeOutput := range cases {
		t.Run(name, func(t *testing.T) {
			if names, ok := yyAdmissionListNames(makeOutput()); ok || len(names) > 0 {
				t.Fatal("unreliable roster accepted", names, ok)
			}
		})
	}
	if names, ok := yyAdmissionListNames(&packet.CommandOutput{SuccessCount: 1, DataSet: `{"players":"observer, human-a"}`}); !ok || len(names) != 2 {
		t.Fatal("explicit dataset roster rejected", names, ok)
	}
}
