package main

import (
	"encoding/hex"
	auth "github.com/Yeah114/FunAuth/auth"
)

// The SDK's TanLobby response decoder expects hex, not JSON's []byte base64.
func tanLobbyWireResult(r auth.TanLobbyLoginResult) map[string]any {
	keys := make([]string, 0, len(r.RoomModEncryptKey))
	for _, key := range r.RoomModEncryptKey {
		keys = append(keys, hex.EncodeToString(key))
	}
	return map[string]any{"user_unique_id": r.UserUniqueID, "user_player_name": r.UserPlayerName, "growth_level": r.BotLevel, "outfit_info": r.BotComponent, "room_owner_id": r.RoomOwnerID, "room_mod_display_name": r.RoomModDisplayName, "room_mod_download_url": r.RoomModDownloadURL, "room_mod_encrypt_key": keys, "raknet_server_address": r.RaknetServerAddress, "raknet_rand": hex.EncodeToString(r.RaknetRand), "raknet_aes_rand": hex.EncodeToString(r.RaknetAESRand), "encrypt_key_bytes": hex.EncodeToString(r.EncryptKeyBytes), "decrypt_key_bytes": hex.EncodeToString(r.DecryptKeyBytes), "signaling_server_address": r.SignalingServerAddress, "signaling_seed": hex.EncodeToString(r.SignalingSeed), "signaling_ticket": hex.EncodeToString(r.SignalingTicket)}
}
