package main

import (
	"fmt"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func wideLocals(n int) string {
	var code strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&code, "local v%d=%d\n", i, i)
	}
	return code.String()
}

func TestWideRegistersExecuteAndKeepState(t *testing.T) {
	for _, n := range []int{201, 300, 1000, 5000, 5120} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			L := newYYLuaVM()
			defer L.Close()
			// Returning the last local exercises the widened A/B fields. The
			// high register captured by a closure verifies upvalue binding.
			code := "function big()\n" + wideLocals(n) + fmt.Sprintf("return function() v%d=v%d+1; return v%d end\nend\nf=big(); assert(f()==%d); assert(f()==%d)", n, n, n, n+1, n+2)
			if err := L.DoString(code); err != nil {
				t.Fatal(err)
			}
			big := L.GetGlobal("big").(*lua.LFunction)
			if int(big.Proto.NumUsedRegisters) <= 200 {
				t.Fatal("did not exercise wide registers")
			}
			if n == 5120 && int(big.Proto.NumUsedRegisters) != 5120 {
				t.Fatalf("boundary must actually use 5120 registers, got %d",big.Proto.NumUsedRegisters)
			}
		})
	}
}

func TestWideRegistersNestedFramesAndArguments(t *testing.T) {
	L := newYYLuaVM()
	defer L.Close()
	code := "function inner()\n" + wideLocals(4000) + "return v4000\nend\nfunction outer()\n" + wideLocals(4000) + "return inner()+v4000\nend\nassert(outer()==8000)"
	if err := L.DoString(code); err != nil {
		t.Fatal(err)
	}
	var args, vals []string
	for i := 1; i <= 1000; i++ {
		args = append(args, fmt.Sprintf("a%d", i))
		vals = append(vals, fmt.Sprint(i))
	}
	code = "function many(" + strings.Join(args, ",") + ") return a1+a1000 end\nassert(many(" + strings.Join(vals, ",") + ")==1001)"
	if err := L.DoString(code); err != nil {
		t.Fatal(err)
	}
}

func TestWideRegisterOverflowIsCompileError(t *testing.T) {
	L := newYYLuaVM()
	defer L.Close()
	_, err := L.LoadString("function tooBig()\n" + wideLocals(5121) + "return v5121\nend")
	if err == nil {
		t.Fatal("must reject over-limit function before executing")
	}
	if !strings.Contains(err.Error(), "5120") {
		t.Fatalf("missing actual limit: %v", err)
	}
	if err := L.DoString("assert(1+1==2)"); err != nil {
		t.Fatal("compile failure damaged VM:", err)
	}
}
