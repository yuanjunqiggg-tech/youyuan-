package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"sync"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Noto Sans CJK is OFL licensed; this subset is generated from official font source.
//
//go:embed assets/qq-assistant/NotoSansCJKsc-subset.otf
var yyQQCardFont []byte
var yyQQCardTypeface *opentype.Font
var yyQQCardFontError error
var yyQQCardFontOnce sync.Once

func yyQQStatsFor(player string) (yyQQPlayerStats, bool) {
	yyQQHelper.Lock()
	defer yyQQHelper.Unlock()
	v := yyQQHelper.Stats[player]
	var s yyQQPlayerStats
	if v != nil {
		s = *v
	}
	online := playerInfoMgr.OnlineSet()[player]
	bm := getBotManagerLocked()
	if bm == nil || !bm.IsConnected() {
		online = false
	}
	if !s.Joined.IsZero() && online {
		s.OnlineSeconds += int64(time.Since(s.Joined).Seconds())
	}
	return s, online
}
func yyQQPlayerSummary(player string) string {
	s, online := yyQQStatsFor(player)
	status := "离线"
	if online {
		status = "在线"
	}
	join := "未记录"
	leave := "未记录"
	if !s.LastJoin.IsZero() {
		join = s.LastJoin.In(time.Local).Format("01-02 15:04:05")
	}
	if !s.LastLeave.IsZero() {
		leave = s.LastLeave.In(time.Local).Format("01-02 15:04:05")
	}
	msg := s.LastMessage
	if msg == "" {
		msg = "未记录"
	}
	return fmt.Sprintf("游戏ID：%s\n状态：%s\n累计观测在线：%d分%d秒\n发言数量：%d\n最近发言：%s\n最近上线：%s\n最近离线：%s\n统计从当前设备开始观测，不代表服务器完整历史。", player, status, s.OnlineSeconds/60, s.OnlineSeconds%60, s.Messages, msg, join, leave)
}
func yyQQPlayerCard(player string) ([]byte, error) {
	if player == "" || len([]rune(player)) > 64 {
		return nil, fmt.Errorf("请输入游戏ID，最多64字")
	}
	yyQQCardFontOnce.Do(func() { yyQQCardTypeface, yyQQCardFontError = opentype.Parse(yyQQCardFont) })
	if yyQQCardFontError != nil {
		return nil, yyQQCardFontError
	}
	img := image.NewRGBA(image.Rect(0, 0, 820, 560))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{242, 247, 247, 255}), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(24, 24, 796, 536), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(24, 24, 796, 38), image.NewUniform(color.RGBA{15, 119, 110, 255}), image.Point{}, draw.Src)
	text := func(x, y int, s string, size float64) {
		face, e := opentype.NewFace(yyQQCardTypeface, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
		if e != nil {
			return
		}
		defer face.Close()
		d := font.Drawer{Dst: img, Src: image.NewUniform(color.RGBA{31, 57, 57, 255}), Face: face, Dot: fixed.P(x, y)}
		d.DrawString(s)
	}
	text(52, 81, "游园 · 玩家资料", 25)
	text(220, 136, yyQQCardClip(player, 17), 28)
	skin, w, h, ok := playerInfoMgr.GetPlayerSkin(player)
	hasSkin := ok && w >= 64 && h >= w/4 && w <= 1024 && h <= 1024 && uint64(len(skin)) >= uint64(w)*uint64(h)*4
	if hasSkin {
		scale := int(w) / 64
		for y := 0; y < 112; y++ {
			for x := 0; x < 112; x++ {
				sx, sy := 8*scale+x*8*scale/112, 8*scale+y*8*scale/112
				i := (sy*int(w) + sx) * 4
				base := color.NRGBA{skin[i], skin[i+1], skin[i+2], skin[i+3]}
				img.Set(64+x, 108+y, base)
				hx := 40*scale + x*8*scale/112
				hi := (sy*int(w) + hx) * 4
				if hx < int(w) && hi+3 < len(skin) && skin[hi+3] > 0 {
					a := float64(skin[hi+3]) / 255
					c := img.RGBAAt(64+x, 108+y)
					img.SetRGBA(64+x, 108+y, color.RGBA{uint8(float64(skin[hi])*a + float64(c.R)*(1-a)), uint8(float64(skin[hi+1])*a + float64(c.G)*(1-a)), uint8(float64(skin[hi+2])*a + float64(c.B)*(1-a)), 255})
				}
			}
		}
	} else {
		draw.Draw(img, image.Rect(64, 108, 176, 220), image.NewUniform(color.RGBA{223, 239, 235, 255}), image.Point{}, draw.Src)
		text(74, 165, "未收到皮肤", 15)
	}
	s, online := yyQQStatsFor(player)
	state := "当前离线"
	if online {
		state = "当前在线"
	}
	text(220, 176, state, 20)
	text(220, 212, fmt.Sprintf("累计观测在线 %d 分 %d 秒", s.OnlineSeconds/60, s.OnlineSeconds%60), 20)
	text(64, 274, fmt.Sprintf("发言数量：%d 条", s.Messages), 22)
	recent := "未记录"
	if !s.LastJoin.IsZero() {
		recent = s.LastJoin.In(time.Local).Format("2006-01-02 15:04:05") + fmt.Sprintf("（%d分钟前）", int(time.Since(s.LastJoin).Minutes()))
	}
	text(64, 316, "最近上线："+recent, 20)
	leave := "未记录"
	if !s.LastLeave.IsZero() {
		leave = s.LastLeave.In(time.Local).Format("2006-01-02 15:04:05")
	}
	text(64, 354, "最近离线："+leave, 20)
	msg := s.LastMessage
	if msg == "" {
		msg = "未记录"
	}
	text(64, 396, "最近发言：", 20)
	r := []rune(msg)
	for n := 0; n < 2 && len(r) > 0; n++ {
		take := min(32, len(r))
		s := string(r[:take])
		r = r[take:]
		if n == 1 && len(r) > 0 {
			s += "…"
		}
		text(64, 430+n*28, s, 19)
	}
	text(64, 507, "本设备的实际观测记录 · 不含玩家坐标", 16)
	var buf bytes.Buffer
	if e := png.Encode(&buf, img); e != nil {
		return nil, e
	}
	return buf.Bytes(), nil
}
func yyQQCardClip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
