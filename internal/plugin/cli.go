package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Main(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	home, err := homeDir()
	if err != nil {
		return fail(err)
	}
	switch args[0] {
	case "init":
		if len(args) != 2 {
			usage()
			return 2
		}
		_, err := inspect(args[1])
		if err != nil {
			return fail(err)
		}
		if _, err := os.Stat(statePath(home)); err == nil {
			return fail(fmt.Errorf("plugin already installed; use mole-zh apply or uninstall"))
		}
		stablePath, err := filepath.Abs(args[1])
		if err != nil {
			return fail(err)
		}
		s := state{OfficialMole: stablePath, Files: make(map[string]fileRecord)}
		if err := apply(home, &s, true); err != nil {
			noteApply(home, err)
			return fail(err)
		}
		noteApply(home, nil)
		if err := installShims(home); err != nil {
			_ = restore(home, &s, true)
			_ = removeShims(home)
			_ = os.Remove(statePath(home))
			return fail(err)
		}
		fmt.Println("插件已安装。请重新打开终端，或运行安装输出中的 PATH 命令。")
		return 0
	case "apply":
		s, err := loadState(home)
		if err != nil {
			return fail(err)
		}
		if err := apply(home, &s, true); err != nil {
			noteApply(home, err)
			return fail(err)
		}
		noteApply(home, nil)
		return 0
	case "status":
		s, err := loadState(home)
		if err != nil {
			return fail(err)
		}
		in, inspectErr := inspect(s.OfficialMole)
		fmt.Printf("官方入口：%s\n", s.OfficialMole)
		fmt.Printf("已汉化版本：V%s；应用 %d 处；未匹配旧规则 %d 条\n", s.Version, s.Applied, len(s.Skipped))
		if inspectErr == nil {
			fmt.Printf("当前官方版本：V%s\n", in.Version)
		} else {
			fmt.Printf("当前官方入口不可用：%v\n", inspectErr)
		}
		if inspectErr != nil || in.Root != s.Root || in.Version != s.Version || s.CatalogHash != digest(rulesData) || !samePatched(s) {
			fmt.Println("状态：官方文件已变化，下次 mo 启动会尝试重新应用汉化。")
		} else {
			fmt.Println("状态：汉化文件完整。")
		}
		if lastError, err := os.ReadFile(filepath.Join(home, "last_error")); err == nil {
			fmt.Printf("最近一次适配失败：%s\n", strings.TrimSpace(string(lastError)))
		}
		limit := 20
		if len(args) > 1 && args[1] == "--all" {
			limit = len(s.Skipped)
		}
		for i, item := range s.Skipped {
			if i >= limit {
				fmt.Printf("其余 %d 条请使用 mole-zh status --all 查看。\n", len(s.Skipped)-limit)
				break
			}
			fmt.Println("未匹配：" + item)
		}
		return 0
	case "uninstall":
		s, err := loadState(home)
		if err != nil {
			return fail(err)
		}
		if err := restore(home, &s, true); err != nil {
			return fail(err)
		}
		if err := removeShims(home); err != nil {
			return fail(err)
		}
		_ = os.Remove(statePath(home))
		_ = os.Remove(filepath.Join(home, "last_error"))
		fmt.Println("汉化插件已卸载；Mole 用户设置保持不变。")
		return 0
	case "run":
		s, err := loadState(home)
		if err != nil {
			return fail(err)
		}
		if os.Getenv("MOLE_ZH_BYPASS") != "1" {
			if err := apply(home, &s, false); err != nil {
				noteApply(home, err)
				fmt.Fprintf(os.Stderr, "mole-zh: 汉化适配失败，继续运行官方 Mole：%v\n", err)
			} else {
				noteApply(home, nil)
			}
		}
		cmd := exec.Command(s.OfficialMole, args[1:]...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		cmd.Env = append(os.Environ(), "MOLE_ZH_BYPASS=1")
		runErr := cmd.Run()
		code := 0
		if runErr != nil {
			if exitErr, ok := runErr.(*exec.ExitError); ok {
				code = exitErr.ExitCode()
			} else {
				return fail(runErr)
			}
		}
		if os.Getenv("MOLE_ZH_BYPASS") != "1" {
			if _, err := os.Stat(s.OfficialMole); os.IsNotExist(err) && len(args) > 1 && args[1] == "remove" && code == 0 {
				_ = removeShims(home)
				_ = os.Remove(statePath(home))
				_ = os.Remove(filepath.Join(home, "last_error"))
			} else if err := apply(home, &s, false); err != nil {
				noteApply(home, err)
				fmt.Fprintf(os.Stderr, "mole-zh: 官方命令已结束，汉化恢复失败：%v\n", err)
			} else {
				noteApply(home, nil)
			}
		}
		return code
	default:
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "用法：mole-zh status [--all] | apply | uninstall")
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "mole-zh:", err)
	return 1
}

func noteApply(home string, err error) {
	path := filepath.Join(home, "last_error")
	if err == nil {
		_ = os.Remove(path)
		return
	}
	_ = os.MkdirAll(home, 0700)
	_ = os.WriteFile(path, []byte(err.Error()+"\n"), 0600)
}

func restore(home string, s *state, promptSudo bool) error {
	return withLock(home, func() error {
		all := make(map[string]fileRecord)
		for target, rec := range s.Retired {
			all[target] = rec
		}
		for target, rec := range s.Files {
			all[target] = rec
		}
		for target, rec := range all {
			current, err := os.ReadFile(target)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			if digest(current) != rec.Hash {
				fmt.Fprintf(os.Stderr, "mole-zh: %s 已被其他程序更新，保留现有文件\n", target)
				continue
			}
			original, err := os.ReadFile(rec.Backup)
			if err != nil {
				return err
			}
			info, err := os.Lstat(target)
			if err != nil {
				return err
			}
			if err := writeTarget(home, target, original, info.Mode().Perm(), promptSudo); err != nil {
				return err
			}
		}
		return nil
	})
}

const profileStart = "# >>> mole-zh >>>"
const profileEnd = "# <<< mole-zh <<<"

func installShims(home string) error {
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		return err
	}
	manager := filepath.Join(home, "manager")
	for _, name := range []string{"mo", "mole", "mole-zh"} {
		command := "run"
		if name == "mole-zh" {
			command = ""
		}
		content := fmt.Sprintf("#!/bin/sh\nexec %s %s \"$@\"\n", shellQuote(manager), command)
		if err := os.WriteFile(filepath.Join(bin, name), []byte(content), 0755); err != nil {
			return err
		}
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	line := fmt.Sprintf("%s\nexport PATH=%s:\"$PATH\"\n%s\n", profileStart, shellQuote(bin), profileEnd)
	for _, name := range []string{".zshrc", ".bash_profile"} {
		path := filepath.Join(userHome, name)
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if strings.Contains(string(data), profileStart) {
			continue
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		if len(data) > 0 && data[len(data)-1] != '\n' {
			_, _ = f.WriteString("\n")
		}
		_, err = f.WriteString(line)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	fmt.Printf("当前终端立即启用：export PATH=%s:\"$PATH\"\n", shellQuote(bin))
	return nil
}

func removeShims(home string) error {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	for _, name := range []string{".zshrc", ".bash_profile"} {
		path := filepath.Join(userHome, name)
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		before := string(data)
		start := strings.Index(before, profileStart)
		end := strings.Index(before, profileEnd)
		if start < 0 || end < start {
			continue
		}
		end += len(profileEnd)
		if end < len(before) && before[end] == '\n' {
			end++
		}
		updated := before[:start] + before[end:]
		if err := os.WriteFile(path, []byte(updated), 0600); err != nil {
			return err
		}
	}
	for _, name := range []string{"mo", "mole", "mole-zh"} {
		_ = os.Remove(filepath.Join(home, "bin", name))
	}
	return nil
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
