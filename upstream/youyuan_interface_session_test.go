package main

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/OmineDev/flowers-for-machines/game_control/game_interface"
	"github.com/OmineDev/flowers-for-machines/game_control/resources_control"
	"github.com/OmineDev/flowers-for-machines/nbt_assigner"
	"github.com/OmineDev/flowers-for-machines/nbt_assigner/nbt_console"
)

func TestGameInterfaceConcurrentFirstUseCreatesOneInstance(t *testing.T) {
	bm := &BotManager{conn: new(minecraft.Conn), resources: new(resources_control.Resources), connected: true}
	want := new(game_interface.GameInterface)
	var calls, owners atomic.Int32
	create := func(*resources_control.Resources, *minecraft.Conn) (*game_interface.GameInterface, error) {
		calls.Add(1)
		time.Sleep(5 * time.Millisecond)
		return want, nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, created, err := bm.loadGameInterface(create)
			if err != nil || got != want {
				t.Errorf("got %p, err %v", got, err)
			}
			if created {
				owners.Add(1)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 || owners.Load() != 1 {
		t.Fatalf("duplicate initializers/maintenance owners: %d/%d", calls.Load(), owners.Load())
	}
	if got, err := bm.getGameInterface(); got != want || err != nil {
		t.Fatal("cached call unexpectedly initialised another interface", err)
	}
}

func TestGameInterfaceDoesNotPublishAfterConnectionReplacement(t *testing.T) {
	bm := &BotManager{conn: new(minecraft.Conn), resources: new(resources_control.Resources), connected: true}
	entered, release, result := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		_, _, err := bm.loadGameInterface(func(*resources_control.Resources, *minecraft.Conn) (*game_interface.GameInterface, error) {
			close(entered)
			<-release
			return new(game_interface.GameInterface), nil
		})
		result <- err
	}()
	<-entered
	changed := make(chan struct{})
	go func() {
		bm.mu.Lock()
		bm.resetGameInterfacesLocked()
		bm.conn, bm.resources = new(minecraft.Conn), new(resources_control.Resources)
		bm.mu.Unlock()
		close(changed)
	}()
	select {
	case <-changed:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("slow initialisation held packet/heartbeat state lock")
	}
	close(release)
	if err := <-result; err == nil || bm.gameInterface != nil {
		t.Fatal("old connection installed its interface into the replacement", err)
	}
	want := new(game_interface.GameInterface)
	got, created, err := bm.loadGameInterface(func(*resources_control.Resources, *minecraft.Conn) (*game_interface.GameInterface, error) {
		return want, nil
	})
	if err != nil || got != want || !created {
		t.Fatal("replacement could not initialise", err)
	}
}

func TestGameInterfaceFailureCanRetry(t *testing.T) {
	bm := &BotManager{conn: new(minecraft.Conn), resources: new(resources_control.Resources), connected: true}
	_, _, err := bm.loadGameInterface(func(*resources_control.Resources, *minecraft.Conn) (*game_interface.GameInterface, error) {
		return nil, errors.New("initialisation failed")
	})
	if err == nil || bm.gameInterface != nil {
		t.Fatal("failed initializer cached")
	}
	_, created, err := bm.loadGameInterface(func(*resources_control.Resources, *minecraft.Conn) (*game_interface.GameInterface, error) {
		return new(game_interface.GameInterface), nil
	})
	if err != nil || !created {
		t.Fatal("failed setup permanently blocked retry", err)
	}
}

func TestNBTConsoleSetupIsSingleAndCannotOutliveWorkspace(t *testing.T) {
	bm := &BotManager{conn: new(minecraft.Conn), connected: true, gameInterface: new(game_interface.GameInterface)}
	var calls atomic.Int32
	want := new(nbt_console.Console)
	create := func(*game_interface.GameInterface, uint8, protocol.BlockPos) (*nbt_console.Console, error) {
		calls.Add(1)
		return want, nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got, err := bm.loadNBTConsole(protocol.BlockPos{1, 2, 3}, create); err != nil || got != want {
				t.Errorf("got %p, err %v", got, err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("concurrent callers prepared multiple NBT workspaces", calls.Load())
	}
	bm.cleanupNBTWorkspace()
	entered, release, result := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		_, err := bm.loadNBTConsole(protocol.BlockPos{1, 2, 3}, func(*game_interface.GameInterface, uint8, protocol.BlockPos) (*nbt_console.Console, error) {
			close(entered)
			<-release
			return want, nil
		})
		result <- err
	}()
	<-entered
	bm.cleanupNBTWorkspace()
	close(release)
	if err := <-result; err == nil || bm.nbtConsole != nil {
		t.Fatal("cancelled workspace resurrected", err)
	}
}

func TestResetDropsAllSessionInterfacesAndOldMaintainer(t *testing.T) {
	old := new(minecraft.Conn)
	bm := &BotManager{conn: old, connected: true, resources: new(resources_control.Resources), gameInterface: new(game_interface.GameInterface), nbtAssigner: new(nbt_assigner.NBTAssigner), nbtConsole: new(nbt_console.Console)}
	m := &yyBotMaintainer{bm: bm, conn: old}
	bm.mu.Lock()
	bm.resetGameInterfacesLocked()
	bm.conn = new(minecraft.Conn)
	bm.mu.Unlock()
	if bm.resources != nil || bm.gameInterface != nil || bm.nbtAssigner != nil || bm.nbtConsole != nil || bm.nbtWorkspaceEpoch == 0 {
		t.Fatal("old interaction state survived reconnect")
	}
	if m.currentSession() {
		t.Fatal("old maintainer follows replacement connection")
	}
	if err := m.TouchMaintainer(nil); err != nil {
		t.Fatal(err)
	}
	m.HandlePacket(&packet.Respawn{State: packet.RespawnStateReadyToSpawn}, nil)
	m.startWatchdog(nil)
	if m.watching.Load() || m.running.Load() {
		t.Fatal("old callback started maintenance after reconnect")
	}
}
