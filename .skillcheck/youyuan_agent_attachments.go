package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

// Preserve binary files, normalize common text BOMs so Android/Windows exports are readable.
func yyAttachmentText(data []byte) string {
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		return string(data[3:])
	}
	if len(data) >= 2 && ((data[0] == 0xff && data[1] == 0xfe) || (data[0] == 0xfe && data[1] == 0xff)) && len(data)%2 == 0 {
		var order binary.ByteOrder = binary.LittleEndian
		if data[0] == 0xfe {
			order = binary.BigEndian
		}
		words := make([]uint16, (len(data)-2)/2)
		for i := range words {
			words[i] = order.Uint16(data[2+i*2 : 4+i*2])
		}
		return string(utf16.Decode(words))
	}
	return string(data)
}

func yyAgentAttachmentPage(text string, offset, limit int) string {
	runes := []rune(text)
	if offset < 0 {
		offset = 0
	}
	if offset > len(runes) {
		offset = len(runes)
	}
	if limit < 1 {
		limit = 6000
	}
	if limit > 20000 {
		limit = 20000
	}
	end := offset + limit
	if end > len(runes) {
		end = len(runes)
	}
	return fmt.Sprintf("[字符 %d-%d / 共 %d；下一页 offset=%d]\n", offset, end, len(runes), end) + string(runes[offset:end])
}
