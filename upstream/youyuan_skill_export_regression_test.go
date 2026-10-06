package main

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestEveryPublishedSkillExportsCompleteZIP(t *testing.T) {
	for _, row := range yyAllSkillCatalog() {
		id := row["id"].(string)
		t.Run(id, func(t *testing.T) {
			raw, e := yySkillArchive(id)
			if e != nil {
				t.Fatal(e)
			}
			z, e := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
			if e != nil || len(z.File) == 0 {
				t.Fatal("invalid skill ZIP", e)
			}
		})
	}
}
