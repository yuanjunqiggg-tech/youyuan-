package main

import (
	"encoding/json"
	"fmt"
	"time"
)

func yyCaptureGameModeValid(mode string) bool {
	switch mode {
	case "", "keep", "survival", "creative", "adventure", "spectator":
		return true
	}
	return false
}

func yyCaptureBehaviorCommands(name string, o yyObserverDimensionOptions) []string {
	if !yyWatchName(name) || !yyCaptureGameModeValid(o.GameMode) {
		return nil
	}
	quoted, _ := json.Marshal(name)
	var commands []string
	if o.GameMode != "" && o.GameMode != "keep" {
		commands = append(commands, "gamemode "+o.GameMode+" "+string(quoted))
	}
	if o.Invisible != nil {
		command := "effect " + string(quoted) + " invisibility 0"
		if *o.Invisible {
			command = "effect " + string(quoted) + " invisibility 120 0 true"
		}
		commands = append(commands, command)
	}
	return commands
}

// The caller holds dimensionMaintenanceMu. Never send appearance commands on
// each camera switch, and bind the refresh to this connection rather than a name.
func yyCaptureApplyBehavior(bm *BotManager, o yyObserverDimensionOptions, send func(string) error) error {
	bm.mu.Lock()
	name, conn := bm.display, bm.conn
	bm.mu.Unlock()
	commands := yyCaptureBehaviorCommands(name, o)
	if len(commands) == 0 {
		return nil
	}
	key := fmt.Sprintf("%p:%q", conn, commands)
	if bm.captureBehaviorKey == key && time.Since(bm.captureBehaviorAt) < time.Minute {
		return nil
	}
	for _, command := range commands {
		if err := send(command); err != nil {
			return err
		}
	}
	bm.captureBehaviorKey, bm.captureBehaviorAt = key, time.Now()
	return nil
}
