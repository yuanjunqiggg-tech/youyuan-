package main

import (
	"github.com/go-gl/mathgl/mgl32"
	"strconv"
	"time"
)

type yyLocatorObservation struct {
	UniqueID int64
	Position mgl32.Vec3
	Known    bool
	At       time.Time
}

// PlayerLocation carries UniqueID and coordinates but no runtime ID, health or
// dimension. Preserve that distinction even when no AddPlayer has been seen.
func yyLocatorSnapshot(s *yySenseState, target string, connected bool) []any {
	uid, parsed := strconv.ParseInt(target, 10, 64)
	name, uuid, xuid := "", "", ""
	for _, identity := range s.Identities {
		if identity.Online && ((parsed == nil && identity.UniqueID == uid) || identity.Name == target) {
			uid = identity.UniqueID
			name, uuid, xuid = identity.Name, identity.UUID, identity.XUID
			parsed = nil
			break
		}
	}
	if parsed != nil {
		return []any{}
	}
	loc, ok := s.Locations[uid]
	if !ok {
		return []any{}
	}
	var pos any
	if loc.Known {
		pos = loc.Position
	}
	return []any{map[string]any{"unique_id": strconv.FormatInt(uid, 10), "name": name, "uuid": uuid, "xuid": xuid, "known": loc.Known, "position": pos, "dimension": nil, "dimension_known": false, "runtime_id": nil, "updated_at": loc.At.Format(time.RFC3339Nano), "stale": !connected || !loc.Known || yyObservationStale(loc.At, true, connected), "source": "PlayerLocation"}}
}
