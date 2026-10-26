package client

import (
	"encoding/json"
	"fmt"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/OmineDev/flowers-for-machines/core/py_rpc"
)

// CopeChallenge ..
func (m *MCPCheckChallengesSolver) CopeChallenge() error {
	err := m.solveMCPCheckChallenges()
	if err != nil {
		return fmt.Errorf("应对验证挑战失败：%v", err)
	}
	return nil
}

// HandleRuntimeChallenge preserves challenge responses that arrive after reader
// handoff. Only the resource reader calls it; there is no competing reader.
func (c *Client) HandleRuntimeChallenge(p *packet.PyRpc) error {
	return (&MCPCheckChallengesSolver{client: c}).onPyRpc(p)
}

// solveMCPCheckChallenges ..
func (m *MCPCheckChallengesSolver) solveMCPCheckChallenges() error {
	cached, err := collectChallengePackets(m.client.connection, m.client.fireCallbacks, m.onPyRpc,
		func() bool { return m.client.getCheckNumEverPassed }, 32767)
	if err != nil {
		return err
	}
	m.client.cachedPacket = cached
	return nil
}

// onPyRpc ..
func (m *MCPCheckChallengesSolver) onPyRpc(p *packet.PyRpc) error {
	// prepare
	conn := m.client.connection
	client := m.client.authClient
	if p.Value == nil {
		return nil
	}

	// unmarshal
	content, err := py_rpc.Unmarshal(p.Value)
	if err != nil {
		return fmt.Errorf("处理 PyRPC 失败：%v", err)
	}

	// do some actions for some specific PyRpc packets
	switch c := content.(type) {
	case *py_rpc.StartType:
		// get data and send packet
		c.Content, err = client.TransferData(c.Content)
		if err != nil {
			return fmt.Errorf("处理 PyRPC 失败：%v", err)
		}
		c.Type = py_rpc.StartTypeResponse
		conn.WritePacket(&packet.PyRpc{
			Value:         py_rpc.Marshal(c),
			OperationType: packet.PyRpcOperationTypeSend,
		})
	case *py_rpc.GetMCPCheckNum:
		// if the challenges has been down,
		// then we do NOTHING
		if m.client.getCheckNumEverPassed {
			break
		}
		// create request to the auth server and get response
		arg, _ := json.Marshal([]any{
			c.FirstArg,
			c.SecondArg.Arg,
			conn.GameData().EntityUniqueID,
		})
		ret, err := client.TransferCheckNum(string(arg))
		if err != nil {
			return fmt.Errorf("处理 PyRPC 失败：%v", err)
		}
		// unmarshal response and adjust the data included
		ret_p := []any{}
		json.Unmarshal([]byte(ret), &ret_p)
		if len(ret_p) > 7 {
			ret6, ok := ret_p[6].(float64)
			if ok {
				ret_p[6] = int64(ret6)
			}
		}
		// send packet and mark this challenges was finished
		conn.WritePacket(&packet.PyRpc{
			Value:         py_rpc.Marshal(&py_rpc.SetMCPCheckNum{ret_p}),
			OperationType: packet.PyRpcOperationTypeSend,
		})
		m.client.getCheckNumEverPassed = true
	}

	// return
	return nil
}
