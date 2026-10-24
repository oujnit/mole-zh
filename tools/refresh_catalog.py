#!/usr/bin/env python3
"""Extract candidate display-text replacements from the reviewed V1.62 fork.

Run only while maintaining the catalog. The released plugin consumes rules.json
and never needs this script or access to the fork.
"""

import difflib
import json
import re
import subprocess
from pathlib import Path

SOURCE = Path(__file__).resolve().parents[2] / "Mole"
BASE = "91a7bf44"
LOCALIZED = "upgrade-origin-main"

# Short-circuit display lines were excluded by the conservative diff filter.
# These V1.62.0 remove previews were checked against the official tag.
MANUAL_RULES = [
    ("lib/manage/remove.sh", '                [[ -f "$install" ]] && echo -e "  ${GRAY}${ICON_LIST} Would remove: ${install}${NC}"', '                [[ -f "$install" ]] && echo -e "  ${GRAY}${ICON_LIST} 将移除：${install}${NC}"'),
    ("lib/manage/remove.sh", '                [[ -f "$alias" ]] && echo -e "  ${GRAY}${ICON_LIST} Would remove: ${alias}${NC}"', '                [[ -f "$alias" ]] && echo -e "  ${GRAY}${ICON_LIST} 将移除：${alias}${NC}"'),
    ("lib/manage/remove.sh", '        [[ -d "$HOME/.cache/mole" ]] && echo -e "  ${GRAY}${ICON_LIST} Would remove: $HOME/.cache/mole${NC}"', '        [[ -d "$HOME/.cache/mole" ]] && echo -e "  ${GRAY}${ICON_LIST} 将移除：$HOME/.cache/mole${NC}"'),
    ("lib/manage/remove.sh", '                echo -e "  ${GRAY}${ICON_LIST} Would move to Trash: $remove_config_dir${NC}"', '                echo -e "  ${GRAY}${ICON_LIST} 将移至废纸篓：$remove_config_dir${NC}"'),
    ("lib/manage/remove.sh", '        [[ -d "$HOME/Library/Logs/mole" ]] && echo "  ${ICON_LIST} $HOME/Library/Logs/mole (kept, operation history)"', '        [[ -d "$HOME/Library/Logs/mole" ]] && echo "  ${ICON_LIST} $HOME/Library/Logs/mole（保留，操作历史）"'),
    ("lib/manage/remove.sh", '    [[ -d "$HOME/Library/Logs/mole" ]] && echo "  ${ICON_LIST} ~/Library/Logs/mole (kept, operation history)"', '    [[ -d "$HOME/Library/Logs/mole" ]] && echo "  ${ICON_LIST} ~/Library/Logs/mole（保留，操作历史）"'),
]

# New router help is literal heredoc text, so carry its exact lines explicitly.
MANUAL_RULES += [('mole', 'Usage: mo update [--force] [--nightly]', '用法：mo update [--force] [--nightly]'),
 ('mole', 'Update Mole to the latest release.', '将 Mole 更新到最新稳定版。'),
 ('mole', 'Options:', '选项：'),
 ('mole',
  '  -f, --force       Force reinstall latest stable version',
  '  -f, --force       强制重装最新稳定版'),
 ('mole',
  '  --nightly         Install latest unreleased main branch build',
  '  --nightly         安装尚未发布的 main 分支构建'),
 ('mole', '  -h, --help        Show this help message', '  -h, --help        显示本帮助信息'),
 ('mole', 'Usage: mo remove [--dry-run]', '用法：mo remove [--dry-run]'),
 ('mole',
  'Remove Mole from this Mac. Operation logs in ~/Library/Logs/mole are kept.',
  '从这台 Mac 移除 Mole。保留 ~/Library/Logs/mole 中的操作日志。'),
 ('mole', '  -n, --dry-run     Preview Mole removal', '  -n, --dry-run     预览 Mole 卸载操作')]


MANUAL_RULES += [('lib/uninstall/batch.sh', '        [[ ${#_review_selected_paths[@]} -eq 0 ]] || removal_note+=", including personal data"', '        [[ ${#_review_selected_paths[@]} -eq 0 ]] || removal_note+="，包含个人数据"')]



# Short-circuit summary fields are UI text and are excluded by the filter.
MANUAL_RULES += [
    ("bin/clean.sh", '            [[ $files_cleaned -gt 0 ]] && stats+=" | Items: $files_cleaned"', '            [[ $files_cleaned -gt 0 ]] && stats+=" | 项目数：$files_cleaned"'),
    ("bin/clean.sh", '            [[ $total_items -gt 0 ]] && stats+=" | Categories: $total_items"', '            [[ $total_items -gt 0 ]] && stats+=" | 分类：$total_items"'),
]


def pairs():
    patch = subprocess.check_output(
        ["git", "diff", "--unified=0", f"{BASE}..{LOCALIZED}", "--", "mole", "bin", "lib", "cmd"],
        cwd=SOURCE,
        text=True,
    )
    path = None
    removed, added = [], []

    def finish():
        if len(removed) == len(added):
            for before, after in zip(removed, added):
                yield path, before, after
        removed.clear()
        added.clear()

    for line in patch.splitlines():
        if line.startswith("+++ b/"):
            yield from finish()
            path = line[6:]
        elif line.startswith("@@"):
            yield from finish()
        elif line.startswith("-") and not line.startswith("---"):
            removed.append(line[1:])
        elif line.startswith("+") and not line.startswith("+++"):
            added.append(line[1:])
    yield from finish()


def is_display_rule(path, before, after):
    if not path or path.endswith("_test.go") or path.startswith("lib/i18n/"):
        return False
    # These files populate JSON/NDJSON values as well as the terminal UI.
    if path.startswith("cmd/status/metrics") or path in ("cmd/analyze/insights.go", "cmd/analyze/main.go"):
        return False
    if not re.search(r"[\u3400-\u9fff]", after) or re.search(r"[\u3400-\u9fff]", before):
        return False
    stripped = before.lstrip()
    if stripped.startswith(("#", "case ", "[[ ", "elif [[ ", "if [[ ")):
        return False
    if "EXPORT_LIST_FILE" in before or "log_operation " in before:
        return False
    if difflib.SequenceMatcher(None, before, after).ratio() < 0.32:
        return stripped.startswith(("echo ", "echo\t", "printf ", "log_", "start_")) or path == "bin/completion.sh"
    return True


def guard_helper_rule():
    # Keep stable English guard keys intact. The line catalog cannot insert a
    # multiline function, so attach the reviewed helper to its unique caller
    # declaration as one valid Bash line.
    source = subprocess.check_output(
        ["git", "show", f"{LOCALIZED}:lib/core/base.sh"], cwd=SOURCE, text=True
    )
    match = re.search(r"^mole_localize_guard_reason\(\) \{\n(.*?)^\}", source, re.M | re.S)
    if match is None:
        raise RuntimeError("reviewed guard translation helper was not found")
    body = " ".join(line.strip() for line in match.group(1).splitlines())
    if not body.endswith("esac"):
        raise RuntimeError("unexpected guard helper structure")
    anchor = "mole_report_guard_stop() {"
    helper = "mole_localize_guard_reason() { " + body + "; }; "
    return "lib/core/base.sh", anchor, helper + anchor


def main():
    seen = {}
    conflicts = []
    for path, before, after in pairs():
        if not is_display_rule(path, before, after):
            continue
        key = (path, before)
        if key in seen and seen[key] != after:
            conflicts.append(key)
        else:
            seen[key] = after
    for key in conflicts:
        seen.pop(key, None)
    for path, before, after in [*MANUAL_RULES, guard_helper_rule()]:
        seen[(path, before)] = after
    rules = [
        {"file": path, "before": before, "after": after}
        for (path, before), after in sorted(seen.items())
    ]
    target = Path(__file__).resolve().parents[1] / "internal/plugin/rules.json"
    target.write_text(json.dumps({"base": "V1.62.0", "rules": rules}, ensure_ascii=False, indent=2) + "\n")
    print(f"{len(rules)} rules; {len(set(conflicts))} ambiguous rules omitted")


if __name__ == "__main__":
    main()
