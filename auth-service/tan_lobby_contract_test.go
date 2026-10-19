package main

import (
	auth "github.com/Yeah114/FunAuth/auth"
	"testing"
)

func TestTanLobbyWireUsesHexContract(t *testing.T) {
	r := tanLobbyWireResult(auth.TanLobbyLoginResult{UserUniqueID: 12, RoomOwnerID: 34, RaknetRand: []byte{0, 10, 255}, RaknetAESRand: []byte{10}, EncryptKeyBytes: []byte{255}, DecryptKeyBytes: []byte{0}, SignalingSeed: []byte{11}, SignalingTicket: []byte{12}, RoomModEncryptKey: [][]byte{{1, 2}}})
	for k, want := range map[string]string{"raknet_rand": "000aff", "raknet_aes_rand": "0a", "encrypt_key_bytes": "ff", "decrypt_key_bytes": "00", "signaling_seed": "0b", "signaling_ticket": "0c"} {
		if r[k] != want {
			t.Fatal(k, r[k])
		}
	}
	if r["room_mod_encrypt_key"].([]string)[0] != "0102" {
		t.Fatal(r)
	}
}
