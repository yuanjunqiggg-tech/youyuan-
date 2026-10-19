package auth

import (
	"context"
	"errors"
	g79 "github.com/Yeah114/g79client"
	"testing"
)

func TestTanLobbySelectionNeverFallsBackToFirst(t *testing.T) {
	rows := []g79.TransferRoomWithNameListItem{{RID: g79.Uncertain{Raw: "10"}, Name: "first"}, {RID: g79.Uncertain{Raw: "20"}, Name: "wanted", RoomUniqueID: "unique-20"}}
	for _, input := range []string{"20", "room_id=20", "unique-20", "wanted"} {
		got, err := selectTanLobbyRoom(rows, input)
		if err != nil || got.RID.String() != "20" {
			t.Fatal(input, got, err)
		}
	}
	for _, input := range []string{"999", "room_id=999", "room_id=wanted", "wan"} {
		if got, err := selectTanLobbyRoom(rows, input); err == nil {
			t.Fatal("wrong first/name fallback", input, got)
		}
	}
	rows = append(rows, g79.TransferRoomWithNameListItem{RID: g79.Uncertain{Raw: "30"}, Name: "wanted"})
	if _, err := selectTanLobbyRoom(rows, "wanted"); err == nil {
		t.Fatal("ambiguous name accepted")
	}
	if got, err := selectTanLobbyRoom(rows, "room_id=20"); err != nil || got.RID.String() != "20" {
		t.Fatal(got, err)
	}
}

func TestTanLobbyCancelledBeforeLookup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := TanLobbyLogin(ctx, nil, TanLobbyLoginParams{RoomID: "123"}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
