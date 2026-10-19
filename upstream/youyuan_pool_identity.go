package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type yyPoolActor struct {
	ID      string `json:"id"`
	Admin   bool   `json:"admin"`
	Scope   string `json:"scope"`
	Expires int64  `json:"expires"`
}

var yyPoolRelayIdentity struct {
	sync.RWMutex
	Secret string
}

func yyPoolBoundActor() yyPoolActor {
	yyIdentity.Lock()
	defer yyIdentity.Unlock()
	u := yyIdentity.Data.User
	id, _ := u["id"].(string)
	role, _ := u["role"].(string)
	legacy, _ := u["legacy_owner"].(bool)
	if legacy {
		id = "owner"
	}
	return yyPoolActor{ID: id, Admin: legacy || role == "admin", Scope: "control"}
}
func yyPoolActorRequest(r *http.Request) (yyPoolActor, error) {
	if a, ok := r.Context().Value(yyPoolActorContext{}).(yyPoolActor); ok {
		return a, nil
	}
	if r.Header.Get("X-YouYuan-Pool-Source") != "" || r.Header.Get("X-YouYuan-Pool-Secret") != "" {
		return yyPoolChildActor(r)
	}
	raw := r.Header.Get("X-YouYuan-Actor")
	sig := r.Header.Get("X-YouYuan-Actor-Signature")
	if raw == "" && sig == "" {
		a := yyPoolBoundActor()
		if a.ID == "" {
			return a, fmt.Errorf("请先绑定游园平台账号")
		}
		return a, nil
	}
	yyPoolRelayIdentity.RLock()
	secret := yyPoolRelayIdentity.Secret
	yyPoolRelayIdentity.RUnlock()
	if secret == "" || len(raw) > 4096 {
		return yyPoolActor{}, fmt.Errorf("云端身份会话未就绪")
	}
	key, e := hex.DecodeString(secret)
	if e != nil {
		return yyPoolActor{}, fmt.Errorf("云端身份密钥无效")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(raw))
	signature, e := hex.DecodeString(sig)
	if e != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return yyPoolActor{}, fmt.Errorf("账号池身份签名无效")
	}
	data, e := base64.RawURLEncoding.DecodeString(raw)
	var actor yyPoolActor
	if e != nil || json.Unmarshal(data, &actor) != nil || actor.ID == "" || actor.Expires < time.Now().UnixMilli() || actor.Expires > time.Now().Add(time.Minute).UnixMilli() {
		return actor, fmt.Errorf("账号池身份已过期或无效")
	}
	return actor, nil
}
func yyPoolOwner(a yyManagedAccount) string {
	if a.OwnerID != "" {
		return a.OwnerID
	}
	return yyPoolBoundActor().ID
}
func yyPoolCanUse(actor yyPoolActor, a yyManagedAccount) bool {
	return actor.ID != "" && (actor.Admin || yyPoolOwner(a) == actor.ID)
}
func yyPoolForwardHeaders(r *http.Request) http.Header {
	h := http.Header{}
	for _, k := range []string{"X-YouYuan-Actor", "X-YouYuan-Actor-Signature", "X-YouYuan-Trace-ID"} {
		if v := r.Header.Get(k); v != "" {
			h.Set(k, v)
		}
	}
	return h
}
func yyPoolInvocationAllowed(r *http.Request, id string) error {
	if !yyPoolHasActor(r) {
		return nil
	}
	actor, e := yyPoolActorRequest(r)
	if e != nil {
		return e
	}
	if id == "main" && (actor.Admin || actor.ID == yyPoolBoundActor().ID) {
		return nil
	}
	a, ok := yyManagedFind(id)
	if !ok || !yyPoolCanUse(actor, a) {
		return fmt.Errorf("该机器人不在你的账号池中")
	}
	return nil
}
func yyPoolAccountID(v string) bool {
	return yyManagedAccountID(v) && v != "main" && !strings.HasPrefix(v, ".")
}
func yyPoolEqualKey(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }
