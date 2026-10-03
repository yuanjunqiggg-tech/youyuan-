package main

import (
	"os"
	"time"
	_ "time/tzdata"
)

// Android native Go processes may lack the device's timezone database/config.
// Keep instants unchanged; convert at formatting time, including older UTC rows.
func yyInitTimeZone() {
	name := os.Getenv("YY_TIMEZONE")
	if name == "" {
		name = "Asia/Shanghai"
	}
	zone, e := time.LoadLocation(name)
	if e != nil {
		zone, _ = time.LoadLocation("Asia/Shanghai")
	}
	time.Local = zone
	_ = os.Setenv("TZ", zone.String())
}
