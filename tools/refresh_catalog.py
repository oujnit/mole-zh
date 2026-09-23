#!/usr/bin/env python3
"""Extract candidate display-text replacements from the reviewed V1.55 fork.

Run only while maintaining the catalog. The released plugin consumes rules.json
and never needs this script or access to the fork.
"""

import difflib
import json
import re
import subprocess
from pathlib import Path

SOURCE = Path(__file__).resolve().parents[2] / "Mole"
BASE = "2415b030"
LOCALIZED = "upgrade-origin-main"

# Short-circuit display lines were excluded by the conservative diff filter.
# These five V1.55.0 remove previews were checked against the official tag.
MANUAL_RULES = [
    ("lib/manage/remove.sh", '                [[ -f "$install" ]] && echo -e "  ${GRAY}${ICON_LIST} Would remove: ${install}${NC}"', '                [[ -f "$install" ]] && echo -e "  ${GRAY}${ICON_LIST} 将移除：${install}${NC}"'),
    ("lib/manage/remove.sh", '                [[ -f "$alias" ]] && echo -e "  ${GRAY}${ICON_LIST} Would remove: ${alias}${NC}"', '                [[ -f "$alias" ]] && echo -e "  ${GRAY}${ICON_LIST} 将移除：${alias}${NC}"'),
    ("lib/manage/remove.sh", '        [[ -d "$HOME/.cache/mole" ]] && echo -e "  ${GRAY}${ICON_LIST} Would remove: $HOME/.cache/mole${NC}"', '        [[ -d "$HOME/.cache/mole" ]] && echo -e "  ${GRAY}${ICON_LIST} 将移除：$HOME/.cache/mole${NC}"'),
    ("lib/manage/remove.sh", '        [[ -d "$HOME/.config/mole" ]] && echo -e "  ${GRAY}${ICON_LIST} Would move to Trash: $HOME/.config/mole${NC}"', '        [[ -d "$HOME/.config/mole" ]] && echo -e "  ${GRAY}${ICON_LIST} 将移至废纸篓：$HOME/.config/mole${NC}"'),
    ("lib/manage/remove.sh", '        [[ -d "$HOME/Library/Logs/mole" ]] && echo -e "  ${GRAY}${ICON_LIST} Would remove: $HOME/Library/Logs/mole${NC}"', '        [[ -d "$HOME/Library/Logs/mole" ]] && echo -e "  ${GRAY}${ICON_LIST} 将移除：$HOME/Library/Logs/mole${NC}"'),
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
    if "mole_localize_guard_reason" in after:
        return False
    if "EXPORT_LIST_FILE" in before or "log_operation " in before:
        return False
    if difflib.SequenceMatcher(None, before, after).ratio() < 0.32:
        return stripped.startswith(("echo ", "echo\t", "printf ", "log_", "start_")) or path == "bin/completion.sh"
    return True


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
    for path, before, after in MANUAL_RULES:
        seen[(path, before)] = after
    rules = [
        {"file": path, "before": before, "after": after}
        for (path, before), after in sorted(seen.items())
    ]
    target = Path(__file__).resolve().parents[1] / "internal/plugin/rules.json"
    target.write_text(json.dumps({"base": "V1.55.0", "rules": rules}, ensure_ascii=False, indent=2) + "\n")
    print(f"{len(rules)} rules; {len(set(conflicts))} ambiguous rules omitted")


if __name__ == "__main__":
    main()
