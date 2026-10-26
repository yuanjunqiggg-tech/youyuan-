package main

import (
	"os"
	"testing"
)

func TestGroundReturnEntryOwnsExactlyOneConditionalWorker(t *testing.T) {
	text, err := os.ReadFile("testdata/chain_ground/main.txt")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := yyParseChains(string(text))
	if err != nil {
		t.Fatal(err)
	}
	gates := 0
	for _, chain := range plan.Chains {
		for _, node := range chain.Nodes {
			if node.Control == nil {
				continue
			}
			c := node.Control
			if c.Kind == "gate" && c.Else == "_雪球菜单_按需20Hz.txt" {
				if len(chain.Nodes) != 1 || chain.Hz != 20 {
					t.Fatal("activity polling must have its own segment")
				}
				continue
			}
			if c.Kind != "gate" || c.Then != "_雪球菜单_回地表模块.txt" || c.Command != "testfor @a[tag=yy_surface_latched,tag=!yy_surface_cancel]" || c.PollMS != 100 {
				t.Fatalf("unexpected control: %+v", c)
			}
			gates++
			if len(chain.Nodes) != 1 || chain.Hz != 20 {
				t.Fatal("return polling must have its own segment")
			}
		}
	}
	if gates != 1 || !plan.Segmented {
		t.Fatal("missing/duplicate return entry", gates)
	}
}

func TestGroundReturnChecksPendingRequestOnceBeforeWholeBatch(t *testing.T) {
	text, err := os.ReadFile("testdata/chain_ground/surface.txt")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := yyParseChains(string(text))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Chains) != 2 {
		t.Fatal("initialization and return batch must remain separate", len(plan.Chains))
	}
	body := plan.Chains[1]
	first := body.Nodes[0].Control
	if body.Hz != 20 || first == nil || first.Kind != "wait" || first.Edge == nil || *first.Edge || first.Command != "testfor @a[tag=yy_surface_latched,tag=!yy_surface_cancel]" {
		t.Fatalf("missing frame admission: %+v", first)
	}
	for _, n := range body.Nodes[1:] {
		if n.Control != nil {
			t.Fatal("unexpected per-command barrier")
		}
	}
}
