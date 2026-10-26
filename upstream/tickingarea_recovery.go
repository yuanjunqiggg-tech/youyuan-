package main

import (
	"fmt"

	"bot-apk/state"
)

// Only recover persisted ownership for this server/dimension; live jobs retain their leases.
func yyRecoverTickingAreas(server, dim string, remove func(state.TickingAreaLease) error) error {
	areas, err := state.ListTickingAreas(server)
	if err != nil {
		return err
	}
	for _, v := range areas {
		if v.Dimension != dim {
			continue
		}
		yyActiveTickingAreas.Lock()
		active := yyActiveTickingAreas.leases[v]
		if !active {
			yyActiveTickingAreas.leases[v] = true
		}
		yyActiveTickingAreas.Unlock()
		if active {
			continue
		}
		err = remove(v)
		if err == nil {
			err = state.ForgetTickingArea(v)
		}
		yyActiveTickingAreas.Lock()
		delete(yyActiveTickingAreas.leases, v)
		yyActiveTickingAreas.Unlock()
		if err != nil {
			return fmt.Errorf("恢复清理常加载区域 %s: %w", v.Name, err)
		}
	}
	return nil
}
