package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"os"
	"strings"
)

// A bounded batch amortises HTTP round trips. Decode before writing, then roll
// back the file and counters on a failure so retrying a sequence cannot duplicate
// an accepted prefix. This is conversion batching, not game command pacing.
func (j *yyBlockVideoJob) appendImages(sequence int, images []string) (map[string]any, error) {
	if sequence != j.Frames {
		return nil, fmt.Errorf("帧序号应为%d；请先读取状态后重试", j.Frames)
	}
	if len(images) < 1 || len(images) > 8 {
		return nil, fmt.Errorf("每批1..8张画面")
	}
	decoded := make([]image.Image, 0, len(images))
	for _, encoded := range images {
		if at := strings.IndexByte(encoded, ','); at >= 0 {
			encoded = encoded[at+1:]
		}
		raw, e := base64.StdEncoding.DecodeString(encoded)
		if e != nil {
			return nil, e
		}
		c, _, e := image.DecodeConfig(bytes.NewReader(raw))
		if e != nil || c.Width < 1 || c.Height < 1 || c.Width*c.Height > 65536 {
			return nil, fmt.Errorf("每帧最多65536像素，请先缩放")
		}
		img, _, e := image.Decode(bytes.NewReader(raw))
		if e != nil {
			return nil, e
		}
		decoded = append(decoded, img)
	}
	frames, commands, size, previous := j.Frames, j.Commands, j.Bytes, j.Previous
	var result map[string]any
	changed, total := 0, 0
	for _, img := range decoded {
		v, e := j.appendFrame(img)
		if e != nil {
			j.Frames, j.Commands, j.Bytes, j.Previous = frames, commands, size, previous
			if rollback := os.Truncate(j.Path, int64(size)); rollback != nil {
				return nil, fmt.Errorf("写入失败且回滚失败：%v；请取消该转换任务", rollback)
			}
			return nil, e
		}
		result = v
		changed += v["changed_blocks"].(int)
		total += v["frame_commands"].(int)
	}
	result["batch_frames"] = len(decoded)
	result["batch_changed_blocks"] = changed
	result["batch_frame_commands"] = total
	return result, nil
}
