package main

import (
	"strings"
	"sync"
	"time"
)

type yyQQCompanionSession struct {
	Until    time.Time
	Sequence uint64
}

var yyQQCompanions = struct {
	sync.Mutex
	Sessions map[string]yyQQCompanionSession
	Epoch    map[string]uint64
	Next     uint64
}{Sessions: map[string]yyQQCompanionSession{}, Epoch: map[string]uint64{}}

// No polling or unsolicited repeat messages: only received events advance a session.
func yyQQCompanionBegin(cf yyQQConfig, user, text string, mentioned bool, now time.Time) (key string, epoch, sequence uint64, initial, active bool) {
	group := yyQQGroupString(cf.GroupID)
	key = group + ":" + user
	yyQQCompanions.Lock()
	defer yyQQCompanions.Unlock()
	for k, s := range yyQQCompanions.Sessions {
		if now.After(s.Until) {
			delete(yyQQCompanions.Sessions, k)
		}
	}
	if cf.Assistant.CompanionOwner != "" && user == cf.Assistant.CompanionOwner {
		yyQQCompanions.Epoch[group]++
		for k := range yyQQCompanions.Sessions {
			if strings.HasPrefix(k, group+":") {
				delete(yyQQCompanions.Sessions, k)
			}
		}
		return
	}
	if !cf.Assistant.Companion || cf.Assistant.Model == "" {
		return
	}
	if strings.HasPrefix(text, ".") || strings.HasPrefix(text, "/") || text == "帮助" || text == "list" {
		return
	}
	old, exists := yyQQCompanions.Sessions[key]
	trigger := mentioned
	for _, word := range cf.Assistant.CompanionKeywords {
		if word != "" && strings.Contains(text, word) {
			trigger = true
		}
	}
	if !exists && !trigger {
		return
	}
	if !exists && len(yyQQCompanions.Sessions) >= 128 {
		return
	}
	initial = !exists
	yyQQCompanions.Next++
	old.Sequence = yyQQCompanions.Next
	old.Until = now.Add(30 * time.Second)
	yyQQCompanions.Sessions[key] = old
	return key, yyQQCompanions.Epoch[group], old.Sequence, initial, true
}
func yyQQCompanionCurrent(cf yyQQConfig, key string, epoch, sequence uint64, now time.Time) bool {
	yyQQCompanions.Lock()
	defer yyQQCompanions.Unlock()
	s, ok := yyQQCompanions.Sessions[key]
	return ok && now.Before(s.Until) && s.Sequence == sequence && yyQQCompanions.Epoch[yyQQGroupString(cf.GroupID)] == epoch
}
func yyQQCompanionHandle(cf yyQQConfig, user, text, id string, mentioned bool) bool {
	key, epoch, seq, initial, ok := yyQQCompanionBegin(cf, user, text, mentioned, time.Now())
	if !ok {
		return false
	}
	go func() {
		if initial {
			timer := time.NewTimer(5 * time.Second)
			defer timer.Stop()
			<-timer.C
		}
		current := func() bool {
			next := yyQQSnapshot()
			return next.Enabled && next.Assistant.Companion && next.Assistant.Model == cf.Assistant.Model && next.Assistant.CompanionOwner == cf.Assistant.CompanionOwner && next.Official.AppID == cf.Official.AppID && yyQQGroupEnabled(next, yyQQGroupString(cf.GroupID)) && yyQQCompanionCurrent(cf, key, epoch, seq, time.Now())
		}
		if !current() {
			return
		}
		// Companion conversation never inherits administrator modification powers.
		chat := cf
		chat.Assistant.Admins = nil
		chat.Assistant.SuperAdmins = nil
		chat.Assistant.Prompt += "\n当前是AI陪聊。你是AI，不能假装是服主本人；自然跟随对话，优先聊天，只有明确的服务器查询才调用工具。"
		answer, e := yyQQAskAI(chat, user, text)
		if !current() {
			return
		}
		if e != nil {
			yyQQLog("AI陪聊失败：" + e.Error())
			return
		}
		yyQQFeedback("[AI陪聊] "+answer, id)
	}()
	return true
}
