package main

import (
	"testing"
)

func TestCallbackQueueConfigurationRequiresReloadAndPersists(t *testing.T) {
	old, path := yyObservationSettings(), yyObservationConfigPath
	defer func() { yyObservationConfig.Store(old); yyObservationConfigPath = path }()
	root := t.TempDir()
	yyLoadObservationSettings(root)
	if _, err := yyConfigureObservationSettings([]any{map[string]any{"event_queue": 256, "priority_queue": 512}}); err != nil {
		t.Fatal(err)
	}
	bm := &BotManager{}
	const id = "callback-queue-policy-test"
	defer yyStopResident(bm, id)
	if err := yyLoadResident(bm, id, `function noop()end`, false); err != nil {
		t.Fatal(err)
	}
	yyLuaMu.Lock()
	resident := yyLuaStates[yyLuaKey{bm, id}]
	yyLuaMu.Unlock()
	if cap(resident.senseQueue) != 256 || cap(resident.priorityQueue) != 512 {
		t.Fatal("configured capacities not applied")
	}
	if _, err := yyConfigureObservationSettings([]any{map[string]any{"event_queue": 1024}}); err != nil {
		t.Fatal(err)
	}
	if cap(resident.senseQueue) != 256 {
		t.Fatal("live queue silently replaced")
	}
	yyObservationConfig.Store(old)
	yyLoadObservationSettings(root)
	if yyObservationSettings().EventQueue != 1024 {
		t.Fatal("capacity not persisted")
	}
	yyStopResident(bm, id)
	if err := yyLoadResident(bm, id, `function noop()end`, false); err != nil {
		t.Fatal(err)
	}
	yyLuaMu.Lock()
	resident = yyLuaStates[yyLuaKey{bm, id}]
	yyLuaMu.Unlock()
	if cap(resident.senseQueue) != 1024 || cap(resident.priorityQueue) != 512 {
		t.Fatal("reload did not apply config")
	}
	if _, err := yyConfigureObservationSettings([]any{map[string]any{"event_queue": 0}}); err == nil {
		t.Fatal("invalid capacity accepted")
	}
}
