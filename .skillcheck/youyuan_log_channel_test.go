package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDisplayLogFloodNeverWaitsForConsumer(t *testing.T) {
	channel := make(chan string, 1)
	channel <- "occupied"
	before := yyDisplayLogDropped.Load()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100000; i++ {
			yyReceiveLog(channel, "display backlog")
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("display queue blocked producer")
	}
	if delta := yyDisplayLogDropped.Load() - before; delta != 100000 {
		t.Fatalf("missing diagnostic drops: %d", delta)
	}
	if got := <-channel; got != "occupied" {
		t.Fatal("queued display line was replaced")
	}
	yyReceiveLog(channel, "after recovery")
	if got := <-channel; got != "after recovery" {
		t.Fatal(got)
	}
}

// Keep all production paths on the nonblocking display writer; selected sends
// are already explicitly nonblocking and are not direct writes.
func TestNoProductionBlockingDisplayWrites(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		fs := token.NewFileSet()
		tree, err := parser.ParseFile(fs, entry.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		selected := map[*ast.SendStmt]bool{}
		ast.Inspect(tree, func(n ast.Node) bool {
			if c, ok := n.(*ast.CommClause); ok {
				if s, ok := c.Comm.(*ast.SendStmt); ok {
					selected[s] = true
				}
			}
			return true
		})
		ast.Inspect(tree, func(n ast.Node) bool {
			if s, ok := n.(*ast.SendStmt); ok && !selected[s] {
				isLog := false
				switch ch := s.Chan.(type) {
				case *ast.Ident:
					isLog = ch.Name == "logCh"
				case *ast.SelectorExpr:
					isLog = ch.Sel.Name == "logCh"
				}
				if isLog {
					t.Errorf("blocking log send at %s", fs.Position(s.Pos()))
				}
			}
			return true
		})
	}
}
