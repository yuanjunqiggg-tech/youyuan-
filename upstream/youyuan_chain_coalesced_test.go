package main

import (
	"context"
	"errors"
	"testing"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

func TestChainCoalescedPreservesOrderWithoutExtraPackets(t *testing.T) {
	packets := []packet.Packet{&packet.SettingsCommand{CommandLine: "first"}, &packet.SettingsCommand{CommandLine: "second"}}
	index := 0
	attempted, err := yyChainFastCoalesced(context.Background(), packets, func(p packet.Packet) error {
		if index >= len(packets) || p != packets[index] {
			t.Fatal("order changed or extra query added")
		}
		index++
		return nil
	})
	if err != nil || !attempted || index != 2 {
		t.Fatalf("attempted=%v writes=%d err=%v", attempted, index, err)
	}
}

func TestChainCoalescedStopsAtFirstUnknownWrite(t *testing.T) {
	failure := errors.New("write result unknown")
	writes := 0
	packets := []packet.Packet{&packet.SettingsCommand{}, &packet.SettingsCommand{}, &packet.SettingsCommand{}}
	attempted, err := yyChainFastCoalesced(context.Background(), packets, func(packet.Packet) error {
		writes++
		if writes == 2 {
			return failure
		}
		return nil
	})
	if !attempted || !errors.Is(err, failure) || writes != 2 {
		t.Fatalf("attempted=%v writes=%d err=%v", attempted, writes, err)
	}
}

func TestChainCoalescedCancellationPreservesPartialAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	writes := 0
	packets := []packet.Packet{&packet.SettingsCommand{}, &packet.SettingsCommand{}}
	attempted, err := yyChainFastCoalesced(ctx, packets, func(packet.Packet) error { writes++; cancel(); return nil })
	if !attempted || !errors.Is(err, context.Canceled) || writes != 1 {
		t.Fatalf("attempted=%v writes=%d err=%v", attempted, writes, err)
	}
	attempted, err = yyChainFastCoalesced(ctx, packets, func(packet.Packet) error { t.Fatal("cancelled call wrote packet"); return nil })
	if attempted || !errors.Is(err, context.Canceled) {
		t.Fatalf("attempted=%v err=%v", attempted, err)
	}
}
