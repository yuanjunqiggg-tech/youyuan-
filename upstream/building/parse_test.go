package building

import (
	"os"
	"testing"
)

func TestParseSchemFiles(t *testing.T) {
	testDir := "/root/prism-toolbox/测试文件"
	entries, err := os.ReadDir(testDir)
	if err != nil {
		t.Skipf("未提供旧版外部 schem 素材目录: %v",err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < 6 || name[len(name)-6:] != ".schem" {
			continue
		}
		info, _ := e.Info()
		// Skip files > 1MB to avoid timeout
		if info != nil && info.Size() > 1<<20 {
			t.Logf("Skip %s (%d MB, too large for quick test)", name, info.Size()>>20)
			continue
		}

		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(testDir + "/" + name)
			if err != nil {
				t.Fatal(err)
			}
			sd, err := ParseSchematic(data)
			if err != nil {
				t.Fatal("ParseSchematic error:", err)
			}
			t.Logf("Size: %d x %d x %d, blocks: %d, types: %d",
				sd.SizeX, sd.SizeY, sd.SizeZ, len(sd.Blocks), countBlockTypes(sd))
		})
	}
}

func countBlockTypes(sd *StructureData) int {
	m := make(map[string]struct{})
	for _, idx := range sd.Blocks {
		m[sd.Palette[idx].Name] = struct{}{}
	}
	return len(m)
}
