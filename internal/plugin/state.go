package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type fileRecord struct {
	Backup string `json:"backup"`
	Hash   string `json:"hash"`
}

type state struct {
	OfficialMole string                `json:"official_mole"`
	Root         string                `json:"root"`
	Version      string                `json:"version"`
	CatalogHash  string                `json:"catalog_hash"`
	Files        map[string]fileRecord `json:"files"`
	Retired      map[string]fileRecord `json:"retired_files,omitempty"`
	Applied      int                   `json:"applied"`
	Skipped      []string              `json:"skipped"`
}

type installation struct {
	Mole    string
	Root    string
	Version string
}

var versionRE = regexp.MustCompile(`(?m)^VERSION="([0-9]+\.[0-9]+(?:\.[0-9]+)?)"$`)
var scriptDirRE = regexp.MustCompile(`(?m)^SCRIPT_DIR=['"]([^'"]+)['"]$`)
var scriptDirLineRE = regexp.MustCompile(`(?m)^SCRIPT_DIR=.*$`)

func homeDir() (string, error) {
	if h := os.Getenv("MOLE_ZH_HOME"); h != "" {
		return filepath.Abs(h)
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(userHome, ".local", "share", "mole-zh"), nil
}

func statePath(home string) string { return filepath.Join(home, "state.json") }

func loadState(home string) (state, error) {
	var s state
	data, err := os.ReadFile(statePath(home))
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(data, &s)
	return s, err
}

func saveState(home string, s state) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(home, ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), statePath(home))
}

func inspect(path string) (installation, error) {
	var in installation
	abs, err := filepath.Abs(path)
	if err != nil {
		return in, err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return in, err
	}
	data, err := os.ReadFile(real)
	if err != nil {
		return in, err
	}
	match := versionRE.FindSubmatch(data)
	if len(match) != 2 {
		return in, fmt.Errorf("%s is not a supported Mole entrypoint", real)
	}
	root := filepath.Dir(real)
	if pinned := scriptDirRE.FindSubmatch(data); len(pinned) == 2 {
		root = string(pinned[1])
	}
	if !filepath.IsAbs(root) {
		return in, fmt.Errorf("Mole module directory is not absolute: %s", root)
	}
	for _, required := range []string{"lib/core/common.sh", "bin/clean.sh", "bin/analyze.sh", "bin/status.sh"} {
		if _, err := os.Stat(filepath.Join(root, required)); err != nil {
			return in, fmt.Errorf("Mole installation is incomplete: %s", required)
		}
	}
	if !strings.HasPrefix(string(match[1]), "1.") {
		return in, fmt.Errorf("Mole %s is outside the supported V1 series", match[1])
	}
	return installation{Mole: real, Root: root, Version: string(match[1])}, nil
}

func digest(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func samePatched(s state) bool {
	if len(s.Files) == 0 {
		return false
	}
	for path, rec := range s.Files {
		data, err := os.ReadFile(path)
		if err != nil || digest(data) != rec.Hash {
			return false
		}
	}
	return true
}
