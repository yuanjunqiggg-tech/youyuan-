package main

// 账号皮肤批量预热。
//
// 为什么要单独一份：yyAccountPrimeSkin 只在「保存 / 启用账号」时跑一次，
// 老账号、或者保存时认证服务正好不通的那些账号，磁盘缓存永远是空的。
// 用户点开账号详情才第一次去取纹理，表现就是「皮肤半天不出来」，
// 也就是「你也没把它提前下载了」。
//
// 这里在账号列表加载时补一次：按认证服务地址分组，一份账号列表复用给
// 这一批账号匹配，取到的纹理落进同一个磁盘缓存（yySkinCacheEnqueue）。
//
// 三条硬约束：
//  1. 必须在 goroutine 里跑，不能拖慢列表请求；
//  2. 每个账号有冷却，失败也会记时间——认证服务不通时不能每 10 秒重打一轮；
//  3. 失败必须写日志，并在账号摘要里回一个 skin_cached，不静默。

import (
	"sync"
	"time"
)

const (
	// 成功过的账号短期内不再问认证服务：皮肤换了会由内核 PlayerSkin 覆盖。
	yyAccountSkinPrimeTTL = 30 * time.Minute
	// 失败的账号也要冷却，否则一次断网会把每个账号重试到打满认证服务。
	yyAccountSkinPrimeFailTTL = 3 * time.Minute
)

var (
	yyAccountSkinPrimeMu      sync.Mutex
	yyAccountSkinPrimeLast    = map[string]time.Time{}
	yyAccountSkinPrimeReasons = map[string]string{}
	yyAccountSkinPrimeBusy    bool
)

// yyAccountSkinCached 报告这个账号是否已经有可用的皮肤纹理（磁盘缓存）。
// 账号摘要把它回给前端，用来区分「已有皮肤」和「还没落盘」。
func yyAccountSkinCached(a yyManagedAccount) bool {
	return yySkinCacheHasAny(yyAccountSkinNames(a))
}

// yyAccountSkinPrimeDue 判断这个账号现在是否该再试一次。
func yyAccountSkinPrimeDue(id string, ok bool) bool {
	yyAccountSkinPrimeMu.Lock()
	defer yyAccountSkinPrimeMu.Unlock()
	last, seen := yyAccountSkinPrimeLast[id]
	if !seen {
		return true
	}
	ttl := yyAccountSkinPrimeFailTTL
	if ok {
		ttl = yyAccountSkinPrimeTTL
	}
	return time.Since(last) >= ttl
}

// yyAccountSkinPrimeMark 记一次预热结果。reason 是失败原因，
// 账号摘要通过 yyAccountSkinPrimeReason 读出来——用户要求「失败不能静默」。
func yyAccountSkinPrimeMark(id string, ok bool, reason string) {
	yyAccountSkinPrimeMu.Lock()
	yyAccountSkinPrimeLast[id] = time.Now()
	if reason == "" {
		delete(yyAccountSkinPrimeReasons, id)
	} else {
		yyAccountSkinPrimeReasons[id] = reason
	}
	yyAccountSkinPrimeMu.Unlock()
}

// yyAccountSkinPrimeReason 返回这个账号最近一次预热失败的原因（没有则空串）。
func yyAccountSkinPrimeReason(id string) string {
	yyAccountSkinPrimeMu.Lock()
	defer yyAccountSkinPrimeMu.Unlock()
	return yyAccountSkinPrimeReasons[id]
}

// yyAccountPrimeSkins 批量预热。同一时刻只允许一批在跑：
// 账号页每 10 秒刷一次，没有这道闸会出现多批并发打同一个认证服务。
func yyAccountPrimeSkins(list []yyManagedAccount) (primed, skipped, failed int) {
	if len(list) == 0 || yySkinDiskRoot == "" {
		return 0, 0, 0
	}
	yyAccountSkinPrimeMu.Lock()
	if yyAccountSkinPrimeBusy {
		yyAccountSkinPrimeMu.Unlock()
		return 0, 0, 0
	}
	yyAccountSkinPrimeBusy = true
	yyAccountSkinPrimeMu.Unlock()
	defer func() {
		yyAccountSkinPrimeMu.Lock()
		yyAccountSkinPrimeBusy = false
		yyAccountSkinPrimeMu.Unlock()
	}()

	// 认证服务地址 -> 账号列表。一个箱子通常只有一个地址，等于整批只打一次。
	type yyAccountSkinRows struct {
		rows   []yyAuthAccountBrief
		reason string
	}
	rowsByOrigin := map[string]yyAccountSkinRows{}
	for _, a := range list {
		if yyAccountSkinCached(a) {
			skipped++
			continue
		}
		if !yyAccountSkinPrimeDue(a.ID, false) {
			skipped++
			continue
		}
		origin := yyAccountSkinOrigin(a)
		cached, seen := rowsByOrigin[origin]
		if !seen {
			rows, reason := yyAccountSkinAuthAccountsBackground(origin)
			cached = yyAccountSkinRows{rows: rows, reason: reason}
			rowsByOrigin[origin] = cached
		}
		if cached.reason != "" {
			failed++
			yyAccountSkinPrimeMark(a.ID, false, cached.reason)
			debugLog("[皮肤] 账号 %s 列表预热失败：%s", a.ID, cached.reason)
			continue
		}
		result := yyAccountSkinFetch(a, cached.rows)
		if !result.OK() {
			failed++
			yyAccountSkinPrimeMark(a.ID, false, result.Reason)
			debugLog("[皮肤] 账号 %s 列表预热失败：%s", a.ID, result.Reason)
			continue
		}
		yyAccountSkinStore(result)
			yyAccountSkinPrimeMark(a.ID, true, "")
		primed++
	}
	if primed > 0 || failed > 0 {
		debugLog("[皮肤] 列表预热：落盘 %d 个，失败 %d 个，跳过 %d 个", primed, failed, skipped)
	}
	return primed, skipped, failed
}
