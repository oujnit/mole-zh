package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogAndExactTranslation(t *testing.T) {
	catalog, err := loadCatalog()
	if err != nil || len(catalog.Rules) < 2000 {
		t.Fatalf("catalog invalid: %v, rules=%d", err, len(catalog.Rules))
	}
	for _, rule := range catalog.Rules {
		if strings.Contains(rule.Before, "EXPORT_LIST_FILE") || strings.Contains(rule.Before, "log_operation ") {
			t.Fatalf("machine-readable or operation log content must not be translated: %s", rule.File)
		}
	}
	rules := []Rule{{File: "mole", Before: `echo "Update Mole"`, After: `echo "更新 Mole"`}}
	input := []byte("echo \"Update Mole\"\necho \"New upstream text\"\necho \"Update Mole\"\n")
	output, count, skipped := translateLines(input, rules)
	if count != 2 || len(skipped) != 0 || strings.Count(string(output), "更新 Mole") != 2 ||
		!strings.Contains(string(output), "New upstream text") {
		t.Fatalf("unexpected translation: count=%d skipped=%v output=%q", count, skipped, output)
	}
	_, count, skipped = translateLines(input, []Rule{{File: "mole", Before: `echo "Changed"`, After: `echo "变化"`}})
	if count != 0 || len(skipped) != 1 {
		t.Fatalf("changed English text must remain untranslated: count=%d skipped=%v", count, skipped)
	}
}

func TestOfficialEntrypointLayouts(t *testing.T) {
	root := t.TempDir()
	for _, mode := range []string{"script", "homebrew"} {
		t.Run(mode, func(t *testing.T) {
			install := filepath.Join(root, mode)
			binaryDir := filepath.Join(install, "bin")
			codeDir := install
			if mode == "script" {
				codeDir = filepath.Join(install, "config")
			}
			for _, rel := range []string{"lib/core/common.sh", "bin/clean.sh", "bin/analyze.sh", "bin/status.sh"} {
				path := filepath.Join(codeDir, rel)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("#!/bin/bash\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll(binaryDir, 0755); err != nil {
				t.Fatal(err)
			}
			mole := filepath.Join(binaryDir, "mole")
			if mode == "homebrew" {
				mole = filepath.Join(install, "mole")
			}
			scriptDir := `SCRIPT_DIR="$(dirname "$SCRIPT_PATH")"`
			if mode == "script" {
				scriptDir = `SCRIPT_DIR="` + codeDir + `"`
			}
			content := []byte("#!/bin/bash\n" + scriptDir + "\nVERSION=\"1.55.0\"\n")
			if err := os.WriteFile(mole, content, 0755); err != nil {
				t.Fatal(err)
			}
			entry := mole
			if mode == "homebrew" {
				entry = filepath.Join(binaryDir, "mole")
				if err := os.Symlink(mole, entry); err != nil {
					t.Fatal(err)
				}
			}
			got, err := inspect(entry)
			expectedRoot := codeDir
			if mode == "homebrew" {
				expectedRoot, _ = filepath.EvalSymlinks(codeDir)
			}
			if err != nil || got.Root != expectedRoot || got.Version != "1.55.0" {
				t.Fatalf("inspect: %+v, %v", got, err)
			}
		})
	}
}

func TestWriteBatchRestoresEarlierFiles(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "a")
	second := filepath.Join(root, "b")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("official"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	stage := map[string]stagedFile{
		first:  {Data: []byte("中文"), Original: []byte("official"), Mode: 0644},
		second: {Data: []byte("中文"), Original: []byte("official"), Mode: 0644},
	}
	writes := 0
	writer := func(_ string, path string, data []byte, _ os.FileMode, _ bool) error {
		writes++
		if path == second {
			return errors.New("simulated failure")
		}
		return os.WriteFile(path, data, 0644)
	}
	if err := writeBatch(root, []string{first, second}, stage, false, writer); err == nil {
		t.Fatal("expected write failure")
	}
	data, err := os.ReadFile(first)
	if err != nil || string(data) != "official" || writes != 3 {
		t.Fatalf("rollback failed: %q, writes=%d, err=%v", data, writes, err)
	}
}
