package plugin

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

func withLock(home string, fn func() error) error {
	if err := os.MkdirAll(home, 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(home, "lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}

func officialSource(home, version string) (string, func(), error) {
	if override := testSourceOverride(); override != "" {
		return override, func() {}, nil
	}
	tmp, err := os.MkdirTemp(home, "source-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(tmp) }
	root := filepath.Join(tmp, "mole")
	cmd := exec.Command("git", "clone", "--quiet", "--depth=1", "--branch", "V"+version,
		"https://github.com/tw93/Mole.git", root)
	if output, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("fetch official Mole V%s: %w: %s", version, err, strings.TrimSpace(string(output)))
	}
	return root, cleanup, nil
}

func sameOfficial(actual, source []byte, file string) bool {
	if file == "mole" {
		return bytes.Equal(scriptDirLineRE.ReplaceAll(actual, []byte("SCRIPT_DIR=INSTALL_PATH")),
			scriptDirLineRE.ReplaceAll(source, []byte("SCRIPT_DIR=INSTALL_PATH")))
	}
	return bytes.Equal(actual, source)
}

type stagedFile struct {
	Data     []byte
	Original []byte
	Mode     os.FileMode
}

func apply(home string, s *state, promptSudo bool) error {
	return withLock(home, func() error {
		current, err := inspect(s.OfficialMole)
		if err != nil {
			return err
		}
		if current.Version == s.Version && current.Root == s.Root && samePatched(*s) {
			return nil
		}
		catalog, err := loadCatalog()
		if err != nil {
			return err
		}
		source, cleanup, err := officialSource(home, current.Version)
		if err != nil {
			return err
		}
		defer cleanup()
		sourceMole, err := os.ReadFile(filepath.Join(source, "mole"))
		if err != nil {
			return err
		}
		match := versionRE.FindSubmatch(sourceMole)
		if len(match) != 2 || string(match[1]) != current.Version {
			return fmt.Errorf("official source version differs from installed V%s", current.Version)
		}

		grouped := make(map[string][]Rule)
		for _, rule := range catalog.Rules {
			grouped[rule.File] = append(grouped[rule.File], rule)
		}
		paths := make([]string, 0, len(grouped))
		for path := range grouped {
			paths = append(paths, path)
		}
		sort.Strings(paths)

		stage := make(map[string]stagedFile)
		var skipped []string
		applied := 0
		goTouched := false
		for _, rel := range paths {
			sourcePath := filepath.Join(source, rel)
			sourceData, readErr := os.ReadFile(sourcePath)
			if readErr != nil {
				for _, rule := range grouped[rel] {
					skipped = append(skipped, rule.File+": file missing in official release")
				}
				continue
			}
			if strings.HasPrefix(rel, "cmd/") {
				translated, count, missing := translateLines(sourceData, grouped[rel])
				if count > 0 {
					if err := os.WriteFile(sourcePath, translated, 0644); err != nil {
						return err
					}
					goTouched = true
				}
				applied += count
				skipped = append(skipped, missing...)
				continue
			}
			target := filepath.Join(current.Root, rel)
			if rel == "mole" {
				target = current.Mole
			}
			info, statErr := os.Lstat(target)
			if statErr != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("installed Mole file is missing or not regular: %s", target)
			}
			actual, err := os.ReadFile(target)
			if err != nil {
				return err
			}
			pristine := actual
			if old, ok := s.Files[target]; ok && digest(actual) == old.Hash {
				pristine, err = os.ReadFile(old.Backup)
				if err != nil {
					return fmt.Errorf("backup for %s is unavailable: %w", target, err)
				}
			}
			if !sameOfficial(pristine, sourceData, rel) {
				return fmt.Errorf("%s differs from the official V%s source; refusing to overwrite it", target, current.Version)
			}
			translated, count, missing := translateLines(pristine, grouped[rel])
			applied += count
			skipped = append(skipped, missing...)
			if count > 0 {
				stage[target] = stagedFile{Data: translated, Original: actual, Mode: info.Mode().Perm()}
			}
		}

		if goTouched {
			for _, name := range []string{"analyze", "status"} {
				target := filepath.Join(current.Root, "bin", name+"-go")
				info, err := os.Lstat(target)
				if err != nil || !info.Mode().IsRegular() {
					return fmt.Errorf("installed Go program is missing or not regular: %s", target)
				}
				original, err := os.ReadFile(target)
				if err != nil {
					return err
				}
				output := filepath.Join(filepath.Dir(source), name+"-go")
				cmd := exec.Command("go", "build", "-o", output, "./cmd/"+name)
				cmd.Dir = source
				if result, err := cmd.CombinedOutput(); err != nil {
					return fmt.Errorf("build %s: %w: %s", name, err, strings.TrimSpace(string(result)))
				}
				translated, err := os.ReadFile(output)
				if err != nil {
					return err
				}
				stage[target] = stagedFile{Data: translated, Original: original, Mode: info.Mode().Perm()}
			}
		}

		if len(stage) == 0 {
			return fmt.Errorf("no translation rule matched the installed V%s release", current.Version)
		}
		for target, item := range stage {
			if !strings.HasSuffix(target, ".sh") && filepath.Base(target) != "mole" {
				continue
			}
			probe, err := os.CreateTemp(home, "syntax-*")
			if err != nil {
				return err
			}
			probePath := probe.Name()
			_, err = probe.Write(item.Data)
			probe.Close()
			if err == nil {
				err = exec.Command("bash", "-n", probePath).Run()
			}
			os.Remove(probePath)
			if err != nil {
				return fmt.Errorf("translated shell syntax failed in %s: %w", target, err)
			}
		}
		if err := os.MkdirAll(filepath.Join(home, "backups"), 0700); err != nil {
			return err
		}
		backupDir, err := os.MkdirTemp(filepath.Join(home, "backups"), current.Version+"-")
		if err != nil {
			return err
		}
		newRecords := make(map[string]fileRecord)
		targets := make([]string, 0, len(stage))
		for target := range stage {
			targets = append(targets, target)
		}
		sort.Strings(targets)
		for _, target := range targets {
			item := stage[target]
			original := item.Original
			if old, ok := s.Files[target]; ok && digest(original) == old.Hash {
				original, err = os.ReadFile(old.Backup)
				if err != nil {
					return err
				}
			}
			backup := filepath.Join(backupDir, digest([]byte(target)))
			if err := os.WriteFile(backup, original, 0600); err != nil {
				return err
			}
			newRecords[target] = fileRecord{Backup: backup, Hash: digest(item.Data)}
		}
		if err := writeBatch(home, targets, stage, promptSudo, writeTarget); err != nil {
			return err
		}
		retired := make(map[string]fileRecord)
		for path, rec := range s.Retired {
			retired[path] = rec
		}
		for path, rec := range s.Files {
			if _, active := newRecords[path]; !active {
				retired[path] = rec
			}
		}
		next := state{OfficialMole: s.OfficialMole, Root: current.Root, Version: current.Version,
			Files: newRecords, Retired: retired, Applied: applied, Skipped: skipped}
		if err := saveState(home, next); err != nil {
			for i := len(targets) - 1; i >= 0; i-- {
				target := targets[i]
				item := stage[target]
				if rollbackErr := writeTarget(home, target, item.Original, item.Mode, promptSudo); rollbackErr != nil {
					return fmt.Errorf("save plugin state failed: %v; rollback %s also failed: %w", err, target, rollbackErr)
				}
			}
			return fmt.Errorf("save plugin state: %w", err)
		}
		*s = next
		fmt.Fprintf(os.Stderr, "mole-zh: Mole V%s 已应用 %d 处翻译，%d 条旧规则未匹配\n", current.Version, applied, len(skipped))
		return nil
	})
}

func writeBatch(home string, targets []string, stage map[string]stagedFile, promptSudo bool,
	writer func(string, string, []byte, os.FileMode, bool) error) error {
	var written []string
	for _, target := range targets {
		item := stage[target]
		if err := writer(home, target, item.Data, item.Mode, promptSudo); err != nil {
			for i := len(written) - 1; i >= 0; i-- {
				oldTarget := written[i]
				oldItem := stage[oldTarget]
				if rollbackErr := writer(home, oldTarget, oldItem.Original, oldItem.Mode, promptSudo); rollbackErr != nil {
					return fmt.Errorf("write %s failed: %v; rollback %s also failed: %w", target, err, oldTarget, rollbackErr)
				}
			}
			return fmt.Errorf("write %s failed; all prior files restored: %w", target, err)
		}
		written = append(written, target)
	}
	return nil
}

func writeTarget(home, target string, data []byte, mode os.FileMode, promptSudo bool) error {
	info, err := os.Lstat(target)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("refusing non-regular target %s", target)
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".mole-zh-*")
	if err == nil {
		defer os.Remove(tmp.Name())
		if _, err = tmp.Write(data); err == nil {
			err = tmp.Chmod(mode)
		}
		if err == nil {
			err = tmp.Sync()
		}
		closeErr := tmp.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			return os.Rename(tmp.Name(), target)
		}
	}
	if !errors.Is(err, os.ErrPermission) && !errors.Is(err, syscall.EACCES) {
		return err
	}
	staged, err := os.CreateTemp(home, "privileged-*")
	if err != nil {
		return err
	}
	defer os.Remove(staged.Name())
	if _, err := staged.Write(data); err != nil {
		staged.Close()
		return err
	}
	if err := staged.Close(); err != nil {
		return err
	}
	privTmp := target + fmt.Sprintf(".mole-zh-%d", os.Getpid())
	args := []string{}
	if !promptSudo {
		args = append(args, "-n")
	}
	args = append(args, "/usr/bin/install", "-m", fmt.Sprintf("%04o", mode.Perm()), staged.Name(), privTmp)
	cmd := exec.Command("sudo", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("administrator access needed for %s: %w", target, err)
	}
	moveArgs := []string{}
	if !promptSudo {
		moveArgs = append(moveArgs, "-n")
	}
	moveArgs = append(moveArgs, "/bin/mv", "-f", privTmp, target)
	cmd = exec.Command("sudo", moveArgs...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("privileged rename %s: %w", target, err)
	}
	return nil
}
