//go:build !android

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	dir := os.Getenv("YY_DATA_DIR")
	if dir == "" {
		base, _ := os.UserConfigDir()
		dir = filepath.Join(base, "YouYuanToolbox")
	}
	port := os.Getenv("YY_PORT")
	if port == "" {
		port = "19429"
	}
	fmt.Print(yyBrandBanner(appVersion, "http://127.0.0.1:"+port))
	if err := StartServer(port, dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
