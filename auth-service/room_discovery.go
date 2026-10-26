package main

import (
	"fmt"
	g79 "github.com/Yeah114/g79client"
	"strconv"
	"strings"
	"sync"
)

// Discovery only queries the account's official listings. It never enters a
// room, starts GameStart, or replaces a running connection.
type roomDiscoveryClient interface {
	SearchRentalServerByName(string) (*g79.SearchRentalServerResponse, error)
	SearchOnlineLobbyRoomByKeyword(string, int, int) (*g79.OnlineLobbyRoomSearchResponse, error)
	GetTransferRoomWithName(string) (*g79.TransferRoomWithNameResponse, error)
}

// 租赁服搜索只返回 owner_id（服主 UUID），官方没有「按 UUID 查昵称」的接口，
// 只能拿该服的玩家列表按 user_id 反查名字。命中结果按 UUID 缓存，避免反复请求。
type rentalOwnerLookup interface {
	SearchRentalServerPlayersByServer(g79.SearchRentalServerPlayersByServerRequest) (*g79.SearchRentalServerPlayersByServerResponse, error)
}

var rentalOwnerNames = struct {
	sync.RWMutex
	byID map[string]string
}{byID: map[string]string{}}

// rentalOwnerLookupsPerSearch 限制一次搜索最多反查几个服主，避免把房间搜索拖慢。
const rentalOwnerLookupsPerSearch = 4

func rentalOwnerName(cli roomDiscoveryClient, serverID, ownerID string) string {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return ""
	}
	key := strings.ToLower(ownerID)
	rentalOwnerNames.RLock()
	cached := rentalOwnerNames.byID[key]
	rentalOwnerNames.RUnlock()
	if cached != "" {
		return cached
	}
	lookup, ok := cli.(rentalOwnerLookup)
	if !ok {
		return ""
	}
	resp, err := lookup.SearchRentalServerPlayersByServer(g79.SearchRentalServerPlayersByServerRequest{ServerID: serverID, Length: 50, Offset: 0})
	if err != nil || resp == nil || resp.Code != 0 {
		return ""
	}
	for _, p := range resp.Entities {
		if !strings.EqualFold(strings.TrimSpace(p.UserID), ownerID) && !strings.EqualFold(strings.TrimSpace(p.EntityID), ownerID) {
			continue
		}
		name := strings.TrimSpace(p.Name)
		if name == "" {
			return ""
		}
		rentalOwnerNames.Lock()
		rentalOwnerNames.byID[key] = name
		rentalOwnerNames.Unlock()
		return name
	}
	return ""
}

func discoverRooms(cli roomDiscoveryClient, keyword string, offset int) map[string]any {
	keyword = strings.TrimSpace(keyword)
	keyword = strings.TrimPrefix(strings.TrimPrefix(keyword, "#"), "@")
	keyword = strings.TrimPrefix(keyword, "LobbyGame:room_id=")
	if keyword == "" || len([]rune(keyword)) > 64 || offset < 0 || offset > 100000 {
		return map[string]any{"ok": false, "error": "房间搜索参数无效"}
	}
	rows := []map[string]any{}
	sources := []map[string]any{}
	addSource := func(kind string, count int, err error, more bool) {
		s := map[string]any{"type": kind, "ok": err == nil, "count": count, "more": more}
		if err != nil {
			s["error"] = "该类别官方查询失败，请检查账号状态后重试"
		}
		sources = append(sources, s)
	}
	rental, err := cli.SearchRentalServerByName(keyword)
	if err == nil && rental.Code != 0 {
		err = fmt.Errorf("rental search code %d", rental.Code)
	}
	count := 0
	if err == nil {
		for _, v := range rental.Entities {
			id := v.EntityID.String()
			target := v.Name.String()
			if id == "" || target == "" {
				continue
			}
			row := map[string]any{"id": "rental:" + id, "type": "rental", "room_id": id, "target": target, "name": v.ServerName, "number": target, "owner_id": v.OwnerID.String(), "cover_url": v.ImageURL, "version": v.McVersion}
			if count < rentalOwnerLookupsPerSearch {
				if owner := rentalOwnerName(cli, id, v.OwnerID.String()); owner != "" {
					row["owner"] = owner
				}
			}
			roomCount(row, "players", v.PlayerCount)
			roomCount(row, "max_players", v.Capacity)
			rows = append(rows, row)
			count++
		}
	}
	addSource("rental", count, err, false)
	lobby, err := cli.SearchOnlineLobbyRoomByKeyword(keyword, 20, offset)
	if err == nil && lobby.Code != 0 {
		err = fmt.Errorf("lobby search code %d", lobby.Code)
	}
	count = 0
	more := false
	if err == nil {
		more = lobby.Total.Int64() > int64(offset+len(lobby.Entities)) || lobby.Total.String() == "" && len(lobby.Entities) >= 20
		for _, v := range lobby.Entities {
			id := v.ID()
			if id == "" {
				continue
			}
			row := map[string]any{"id": "lobby:" + id, "type": "lobby", "room_id": id, "target": "LobbyGame:room_id=" + id, "name": v.RoomName, "owner": v.OwnerName, "owner_id": v.OwnerID.String(), "version": v.Version}
			roomCount(row, "players", v.CurNum)
			roomCount(row, "max_players", v.MaxCount)
			rows = append(rows, row)
			count++
		}
	}
	addSource("lobby", count, err, more)
	local, err := cli.GetTransferRoomWithName(keyword)
	if err == nil && local.Code != 0 {
		err = fmt.Errorf("local search code %d", local.Code)
	}
	count = 0
	if err == nil {
		for _, v := range local.List {
			id := v.RID.String()
			if id == "" {
				continue
			}
			row := map[string]any{"id": "local:" + id, "type": "local", "room_id": id, "target": "@" + id, "name": v.Name, "owner_id": v.HID.String()}
			roomCount(row, "players", v.Cnt)
			roomCount(row, "max_players", v.Cap)
			rows = append(rows, row)
			count++
		}
	}
	addSource("local", count, err, false)
	complete := true
	for _, s := range sources {
		if s["ok"] != true || s["more"] == true {
			complete = false
		}
	}
	return map[string]any{"ok": true, "query": keyword, "candidates": rows, "sources": sources, "complete": complete, "offset": offset, "read_only": true}
}

func roomCount(row map[string]any, key string, value g79.Uncertain) {
	if count, err := strconv.ParseInt(value.String(), 10, 64); err == nil && count >= 0 {
		row[key] = count
	}
}
