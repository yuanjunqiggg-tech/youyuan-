package main

import (
	"fmt"
	g79 "github.com/Yeah114/g79client"
	"testing"
)

type roomFixture struct {
	keywords []string
	failed   bool
	more     bool
}

func (f *roomFixture) SearchRentalServerByName(k string) (*g79.SearchRentalServerResponse, error) {
	f.keywords = append(f.keywords, k)
	return &g79.SearchRentalServerResponse{Entities: []g79.RentalServerEntity{{EntityID: g79.Uncertain{Raw: "r1"}, Name: g79.Uncertain{Raw: "114514"}, ServerName: "Rental", PlayerCount: g79.Uncertain{Raw: "0"}}}}, nil
}
func (f *roomFixture) SearchOnlineLobbyRoomByKeyword(k string, n, o int) (*g79.OnlineLobbyRoomSearchResponse, error) {
	f.keywords = append(f.keywords, k)
	if f.failed {
		return nil, fmt.Errorf("fixture error")
	}
	total := "2"
	if f.more {
		total = "50"
	}
	return &g79.OnlineLobbyRoomSearchResponse{Entities: []g79.OnlineLobbyRoomSearchEntity{{RoomID: g79.Uncertain{Raw: "4691202525342713927"}, RoomName: "Lobby one"}, {RoomID: g79.Uncertain{Raw: "4691202525342713928"}, RoomName: "Lobby two"}}, Total: g79.Uncertain{Raw: total}}, nil
}
func (f *roomFixture) GetTransferRoomWithName(k string) (*g79.TransferRoomWithNameResponse, error) {
	f.keywords = append(f.keywords, k)
	return &g79.TransferRoomWithNameResponse{List: []g79.TransferRoomWithNameListItem{{RID: g79.Uncertain{Raw: "5678"}, Name: "Local"}}}, nil
}
func TestRoomDiscoverAllCategoriesAndShortNumber(t *testing.T) {
	f := &roomFixture{}
	r := discoverRooms(f, "#114514", 0)
	if r["complete"] != true {
		t.Fatal(r)
	}
	rows := r["candidates"].([]map[string]any)
	if len(rows) != 4 {
		t.Fatal(rows)
	}
	for _, k := range f.keywords {
		if k != "114514" {
			t.Fatal(f.keywords)
		}
	}
	if rows[1]["room_id"] != "4691202525342713927" {
		t.Fatal("ID lost precision", rows)
	}
	if _, ok := rows[3]["players"]; ok {
		t.Fatal("missing player count invented", rows[3])
	}
	if rows[0]["players"] != int64(0) {
		t.Fatal("actual zero lost")
	}
	if _, ok := rows[1]["avatar_url"]; ok {
		t.Fatal("invented avatar")
	}
}
func TestRoomDiscoverPartialAndPagination(t *testing.T) {
	for _, f := range []*roomFixture{{failed: true}, {more: true}} {
		r := discoverRooms(f, "114514", 0)
		if r["complete"] != false || r["ok"] != true {
			t.Fatal(r)
		}
	}
	if discoverRooms(&roomFixture{}, "", 0)["ok"] != false {
		t.Fatal("empty query accepted")
	}
}
