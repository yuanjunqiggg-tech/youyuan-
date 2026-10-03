package main

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	lua "github.com/yuin/gopher-lua"

	"bot-apk/media"
)

// registerMediaAPI 注册媒体处理 API（图片→ASCII、MIDI 解析、音高换算，纯函数）。
func registerMediaAPI(L *lua.LState) {
	m := L.NewTable()
	L.SetGlobal("media", m)

	// media.imageToAscii(path, maxW) → 字符串数组
	L.SetField(m, "imageToAscii", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		maxW := L.OptInt(2, 60)
		f, err := os.Open(path)
		if err != nil {
			L.RaiseError("media.imageToAscii: %v", err)
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			L.RaiseError("media.imageToAscii: 解码失败: %v", err)
		}
		lines, err := media.ImageToASCII(img, maxW)
		if err != nil {
			L.RaiseError("media.imageToAscii: %v", err)
		}
		t := L.NewTable()
		for i, ln := range lines {
			t.RawSetInt(i+1, lua.LString(ln))
		}
		L.Push(t)
		return 1
	}))

	// media.noteToPitch(note) → 音高频率
	L.SetField(m, "noteToPitch", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(media.NoteToPitch(uint8(L.CheckNumber(1)))))
		return 1
	}))

	// media.parseMidi(path) → {format, tracks, division, tempo, notes=[{tick,channel,note,velocity,duration,sound}]}
	L.SetField(m, "parseMidi", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		data, err := os.ReadFile(path)
		if err != nil {
			L.RaiseError("media.parseMidi: %v", err)
		}
		mf, err := media.ParseMIDIFile(data)
		if err != nil {
			L.RaiseError("media.parseMidi: %v", err)
		}
		t := L.NewTable()
		t.RawSetString("format", lua.LNumber(mf.Format))
		t.RawSetString("tracks", lua.LNumber(mf.Tracks))
		t.RawSetString("division", lua.LNumber(mf.Division))
		t.RawSetString("tempo", lua.LNumber(mf.Tempo))
		notes := L.NewTable()
		for i, n := range mf.Notes {
			nt := L.NewTable()
			nt.RawSetString("tick", lua.LNumber(n.Tick))
			nt.RawSetString("channel", lua.LNumber(n.Channel))
			nt.RawSetString("note", lua.LNumber(n.Note))
			nt.RawSetString("velocity", lua.LNumber(n.Velocity))
			nt.RawSetString("duration", lua.LNumber(n.Duration))
			if n.Sound != "" {
				nt.RawSetString("sound", lua.LString(n.Sound))
			}
			notes.RawSetInt(i+1, nt)
		}
		t.RawSetString("notes", notes)
		L.Push(t)
		return 1
	}))
}
