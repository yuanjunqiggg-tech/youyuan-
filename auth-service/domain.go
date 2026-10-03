package main

import (
	"fmt"
	auth "github.com/Yeah114/FunAuth/auth"
	g79 "github.com/Yeah114/g79client"
)

// Joining preserves all existing memberships. Ambiguous existing memberships
// require an explicit server ID rather than guessing which server to enter.
func safeDomainLogin(cli *g79.Client, invite, selectedID, key string) (auth.LoginResult, error) {
	result := auth.LoginResult{}
	before, e := cli.GetOtherDomainServers()
	if e != nil {
		return result, e
	}
	if before.Code != 0 {
		return result, fmt.Errorf("读取山头列表失败(%d)", before.Code)
	}
	after := before
	joinedID := ""
	alreadyJoined := false
	if invite != "" {
		joined, err := cli.JoinDomainServerWithInviteCode(invite)
		if err != nil {
			return result, err
		}
		if joined.Code != 0 && joined.Code != 1005 {
			return result, fmt.Errorf("山头邀请失败：%s(%d)", joined.Message, joined.Code)
		}
		alreadyJoined = joined.Code == 1005
		for _, v := range joined.Entities {
			if sid, ok := v["sid"].(string); ok && sid != "" {
				joinedID = sid
				break
			}
		}
		after, e = cli.GetOtherDomainServers()
		if e != nil {
			return result, e
		}
		if after.Code != 0 {
			return result, fmt.Errorf("读取山头列表失败(%d)", after.Code)
		}
	}
	sid := selectedID
	if sid == "" {
		sid = joinedID
	}
	if sid == "" {
		old := map[string]bool{}
		for _, v := range before.Entities {
			old[v.Sid] = true
		}
		candidates := []string{}
		for _, v := range after.Entities {
			if !old[v.Sid] {
				candidates = append(candidates, v.Sid)
			}
		}
		if len(candidates) == 1 {
			sid = candidates[0]
		} else if alreadyJoined && len(after.Entities) == 1 {
			sid = after.Entities[0].Sid
		}
	}
	if sid == "" {
		return result, fmt.Errorf("目标山头有歧义，请从已加入列表明确选择server_id")
	}
	member := false
	for _, v := range after.Entities {
		if v.Sid == sid {
			member = true
		}
	}
	if !member {
		return result, fmt.Errorf("所选server_id不在当前账号的山头列表")
	}
	enter, e := cli.RequestEnterDomainServer(sid)
	if e != nil {
		return result, e
	}
	if enter.Code != 0 {
		return result, fmt.Errorf("进入山头失败：%s(%d)", enter.Message, enter.Code)
	}
	data, e := cli.GenerateDomainGameAuthV2(sid, key)
	if e != nil {
		return result, e
	}
	chain, e := cli.SendAuthV2Request(data)
	if e != nil {
		return result, e
	}
	result.UID = cli.UserID
	result.EntityID = cli.UserDetail.EntityID
	result.MasterName = cli.UserDetail.Name
	result.BotLevel = int(cli.UserDetail.Level.Int64())
	result.EngineVersion = cli.EngineVersion
	result.PatchVersion = cli.G79LatestVersion
	result.IP = fmt.Sprintf("%s:%d", enter.Entity.ServerHost, enter.Entity.ServerPort.Int64())
	result.ChainInfo = string(chain)
	return result, nil
}
