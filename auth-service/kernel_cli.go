//go:build !js

package main

import (
	"encoding/json"
	"io"
	"os"
)

func main() {
	raw, e := io.ReadAll(io.LimitReader(os.Stdin, 2<<20))
	if e != nil {
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(kernel(string(raw)))
}
