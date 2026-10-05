# media · 图片与音乐

图片字符画、MIDI解析、音高换算；解析函数本身不自动向游戏播放。


### `media.imageToAscii`

```python
host.media.imageToAscii(path, maxW)
```

- 作用：把图片文件转成 ASCII 字符画行数组。
- 参数：`path` string 必填；`maxW` number 可选，默认 60。
- 返回：对象或数组（字符串数组，1 起索引，每行一个 ASCII 串）。
- 报错：打开/解码/转换失败抛 `media.imageToAscii: <err>`。
- 底层：`os.Open` + `image.Decode`（支持 gif/jpeg/png）→ `media.ImageToASCII(img, maxW)`。

### `media.noteToPitch`

```python
host.media.noteToPitch(note)
```

- 作用：把 MIDI 音符号换算为频率（Hz）。
- 参数：`note` number 必填（强转 `uint8`）。
- 返回：number（音高频率）。
- 报错：非数值抛类型错误。
- 底层：`media.NoteToPitch(uint8(note))`。

### `media.parseMidi`

```python
host.media.parseMidi(path)
```

- 作用：解析 MIDI 文件，返回格式/轨道/分辨率/速度及音符列表。
- 参数：`path` string 必填。
- 返回：对象或数组 `{format, tracks, division, tempo, notes={{tick, channel, note, velocity, duration, sound}}}`（`sound` 仅非空时出现）。
- 报错：读文件/解析失败抛 `media.parseMidi: <err>`。
- 底层：`os.ReadFile` → `media.ParseMIDIFile(data)`。
