package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

const yyProtocolSampleLimit = 512 * 1024

// Opt-in metadata-only evidence. Never sample authentication, chat or inventory frames.
func yyProtocolMetadataSampler(root string) func(packet.Header, []byte, net.Addr, net.Addr) {
	if root == "" {
		return nil
	}
	var mu sync.Mutex
	seen := map[uint32]bool{}
	return func(header packet.Header, payload []byte, _, _ net.Addr) {
		id := header.PacketID
		if id != packet.IDGameRulesChanged && id != packet.IDBiomeDefinitionList {
			return
		}
		mu.Lock()
		if seen[id] {
			mu.Unlock()
			return
		}
		seen[id] = true
		mu.Unlock()
		name := fmt.Sprintf("%d.bin", id)
		var data []byte
		if len(payload) > yyProtocolSampleLimit {
			name = fmt.Sprintf("%d.oversized", id)
			data = []byte(fmt.Sprintf("%d", len(payload)))
		} else {
			data = append([]byte(nil), payload...)
		}
		go func() {
			if os.MkdirAll(root, 0700) != nil {
				return
			}
			file, err := os.OpenFile(filepath.Join(root, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				return
			}
			_, writeErr := file.Write(data)
			closeErr := file.Close()
			if writeErr != nil || closeErr != nil {
				debugLog("[协议取证] 世界元数据样本写入失败")
			}
		}()
	}
}
