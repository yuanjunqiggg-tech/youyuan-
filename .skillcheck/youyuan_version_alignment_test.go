package main

import (
	"os"
	"regexp"
	"strconv"
	"testing"
)

func TestProductVersionMatchesAndroidUpdateCode(t *testing.T) {
	text, err := os.ReadFile("android/app/build.gradle")
	if err != nil {
		t.Fatal(err)
	}
	matches := regexp.MustCompile(`versionCode\s+(\d+);\s+versionName\s+'([^']+)'`).FindSubmatch(text)
	if len(matches) != 3 {
		t.Fatal("Android version declaration missing")
	}
	code, err := strconv.Atoi(string(matches[1]))
	if err != nil {
		t.Fatal(err)
	}
	if code != yyAppVersionCode || string(matches[2]) != yyAppVersion {
		t.Fatalf("update loop/version mismatch: Go %s/%d Android %s/%d", yyAppVersion, yyAppVersionCode, matches[2], code)
	}
}
