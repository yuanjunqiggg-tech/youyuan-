package main

import (
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/google/uuid"
)

func (bm *BotManager) acceptPermissionReceipt(p *packet.CommandOutput) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if bm.sayTestMsg != "" && bm.sayTestRequestID != uuid.Nil && p.SuccessCount > 0 && p.CommandOrigin.UUID == bm.sayTestRequestID {
		bm.sayTestMsg = ""
	}
}

func (bm *BotManager) confirmCommandPermission(perm uint32, source string) bool {
	bm.setOP(true, perm, source)
	if !bm.IsOP() {
		return false
	}
	if bm.onReady != nil {
		bm.onReady()
	}
	return true
}
