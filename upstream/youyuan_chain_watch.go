package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Stat is cheap and happens once a second, not once per Minecraft tick. Source
// is read and compiled only after a change; the current round keeps its IR.
type yyChainStamp struct {
	MTime int64
	Size  int64
}

var yyChainWatch = struct {
	sync.Mutex
	Stamps map[string]yyChainStamp
}{Stamps: map[string]yyChainStamp{}}

func yyReloadChainResource(in *yyChainInstance) error {
	in.Lock()
	name := in.Config.Name
	in.Unlock()
	path := filepath.Join(yyChainRoot, name)
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	plan, e := yyParseChains(string(b))
	if e != nil {
		return fmt.Errorf("%s: %w；保留原运行链", name, e)
	}
	in.Lock()
	in.Plan = plan
	in.Revision++
	in.LastError = ""
	in.Unlock()
	return nil
}

func yyStartChainWatcher() {
	newSchedJobWith("__chainswatch__", "resource_watch", time.Second, false, nil, func(<-chan struct{}) error {
		yyChains.Lock()
		rows := make([]*yyChainInstance, 0, len(yyChains.items))
		for _, in := range yyChains.items {
			rows = append(rows, in)
		}
		yyChains.Unlock()
		for _, in := range rows {
			in.Lock()
			name := in.Config.Name
			in.Unlock()
			st, e := os.Stat(filepath.Join(yyChainRoot, name))
			if e != nil {
				continue
			}
			stamp := yyChainStamp{st.ModTime().UnixNano(), st.Size()}
			yyChainWatch.Lock()
			prev, known := yyChainWatch.Stamps[name]
			yyChainWatch.Stamps[name] = stamp
			yyChainWatch.Unlock()
			if known && prev == stamp {
				continue
			}
			if e = yyReloadChainResource(in); e != nil {
				in.Lock()
				in.LastError = e.Error()
				in.Unlock()
			}
		}
		return nil
	}, nil)
}
