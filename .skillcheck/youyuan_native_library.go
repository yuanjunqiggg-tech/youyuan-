package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
)

func yyValidateNative(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	header := make([]byte, 64)
	if _, e = f.Read(header); e != nil {
		return e
	}
	if string(header[:4]) != "\x7fELF" || header[4] != 2 || header[5] != 1 || binary.LittleEndian.Uint16(header[16:18]) != 3 {
		return fmt.Errorf("需要 64 位小端 ELF 动态库")
	}
	machine := binary.LittleEndian.Uint16(header[18:20])
	expected := uint16(62)
	if runtime.GOARCH == "arm64" {
		expected = 183
	}
	if machine != expected {
		return fmt.Errorf("原生库 ABI 不匹配：当前 %s，库 machine=%d", runtime.GOARCH, machine)
	}
	return nil
}
