#!/bin/bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
gomodcache="$(go env GOMODCACHE)"
gocache="$(go env GOCACHE)"
export GOMODCACHE="$gomodcache"
export GOCACHE="$gocache"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/mole-zh-integration.XXXXXX")"
cleanup_test() {
    local exit_status=$?
    local log_path
    if [[ $exit_status -ne 0 ]]; then
        for log_path in "$test_root"/*.log; do
            if [[ -f "$log_path" ]]; then
                echo "--- $log_path" >&2
                cat "$log_path" >&2
            fi
        done
    fi
    rm -rf "$test_root"
}
trap cleanup_test EXIT
mkdir -p "$test_root/pristine" "$test_root/home"

if [[ -n "${MOLE_ZH_TEST_REPO:-}" ]]; then
    git -C "$MOLE_ZH_TEST_REPO" archive V1.55.0 | tar -x -C "$test_root/pristine"
else
    git clone --quiet --depth=1 --branch V1.55.0 https://github.com/tw93/Mole.git "$test_root/pristine"
fi

export HOME="$test_root/home"
export MOLE_ZH_HOME="$HOME/plugin"
mkdir -p "$MOLE_ZH_HOME"
go build -tags=testsource -o "$MOLE_ZH_HOME/manager" "$repo_root/cmd/mole-zh"

new_source() {
    local name="$1"
    cp -R "$test_root/pristine" "$test_root/$name"
    export MOLE_ZH_SOURCE="$test_root/$name"
}

new_source source-script
mkdir -p "$test_root/script/bin" "$test_root/script/config"
cp "$test_root/pristine/mole" "$test_root/script/bin/mole"
cp -R "$test_root/pristine/bin" "$test_root/script/config/bin"
cp -R "$test_root/pristine/lib" "$test_root/script/config/lib"
(cd "$test_root/pristine" && go build -o "$test_root/script/config/bin/analyze-go" ./cmd/analyze && go build -o "$test_root/script/config/bin/status-go" ./cmd/status)
sed -i '' "s|^SCRIPT_DIR=.*|SCRIPT_DIR=\"$test_root/script/config\"|" "$test_root/script/bin/mole"
original_hash="$(shasum -a 256 "$test_root/script/bin/mole")"
"$test_root/script/config/bin/status-go" --json > "$test_root/status-before.json"
"$MOLE_ZH_HOME/manager" init "$test_root/script/bin/mole" > "$test_root/install.log" 2>&1
"$MOLE_ZH_HOME/bin/mo" --help > "$test_root/help.txt"
grep -q '清理磁盘空间' "$test_root/help.txt"
"$MOLE_ZH_HOME/bin/mole-zh" status > "$test_root/status.txt"
grep -q 'V1.55.0' "$test_root/status.txt"
# Reinstalling a newer plugin must notice changed catalog rules and reapply.
python3 - "$MOLE_ZH_HOME/state.json" <<'PY'
import json, sys
path = sys.argv[1]
state = json.load(open(path))
state['catalog_hash'] = 'previous-catalog'
with open(path, 'w') as output:
    json.dump(state, output)
PY
GOFLAGS=-tags=testsource "$repo_root/install.sh" > "$test_root/reinstall.log" 2>&1
grep -q '已应用' "$test_root/reinstall.log"
grep -q '汉化插件已更新' "$test_root/reinstall.log"
"$MOLE_ZH_HOME/bin/mo" status --json > "$test_root/status.json"
python3 - "$test_root/status-before.json" "$test_root/status.json" <<'PY'
import json, sys
before = json.load(open(sys.argv[1]))
after = json.load(open(sys.argv[2]))
assert before.keys() == after.keys(), (before.keys(), after.keys())
PY
MOLE_TEST_NO_AUTH=1 "$MOLE_ZH_HOME/bin/mo" clean --dry-run > "$test_root/clean-preview.txt"
grep -q '预览' "$test_root/clean-preview.txt"
MOLE_TEST_NO_AUTH=1 "$MOLE_ZH_HOME/bin/mo" remove --dry-run > "$test_root/remove-preview.txt"
grep -q '将移除' "$test_root/remove-preview.txt"

# A direct official reinstall replaces one translated file. The shim restores it.
cp "$test_root/pristine/lib/core/help.sh" "$test_root/script/config/lib/core/help.sh"
"$MOLE_ZH_HOME/bin/mo" --help > "$test_root/help-again.txt" 2> "$test_root/reapply.log"
grep -q '已应用' "$test_root/reapply.log"

# A changed upstream sentence remains English; unrelated translations survive.
new_source source-text-drift
python3 - "$MOLE_ZH_SOURCE/lib/core/help.sh" <<'PY'
from pathlib import Path
import sys
path = Path(sys.argv[1])
source = path.read_text()
old = "Clean up disk space by removing caches, logs, temporary files, and app leftovers from already-uninstalled apps."
assert old in source
path.write_text(source.replace(old, "Clean your Mac using the latest upstream wording."))
PY
cp "$MOLE_ZH_SOURCE/lib/core/help.sh" "$test_root/script/config/lib/core/help.sh"
"$MOLE_ZH_HOME/bin/mo" --help > /dev/null 2> "$test_root/text-drift.log"
grep -q 'Clean your Mac using the latest upstream wording.' "$test_root/script/config/lib/core/help.sh"
grep -q '已应用' "$test_root/text-drift.log"

# An unknown local edit must remain untouched while the official command runs.
printf '\n# local change\n' >> "$test_root/script/config/lib/core/help.sh"
changed_hash="$(shasum -a 256 "$test_root/script/config/lib/core/help.sh")"
"$MOLE_ZH_HOME/bin/mo" --help > /dev/null 2> "$test_root/conflict.log"
grep -q '继续运行官方 Mole' "$test_root/conflict.log"
[[ "$(shasum -a 256 "$test_root/script/config/lib/core/help.sh")" == "$changed_hash" ]]

# Plugin removal restores only files still bearing the plugin's hash.
"$MOLE_ZH_HOME/bin/mole-zh" uninstall > /dev/null
[[ "$(shasum -a 256 "$test_root/script/bin/mole")" == "$original_hash" ]]
[[ ! -e "$MOLE_ZH_HOME/bin/mo" ]]

# Homebrew-style libexec layout and symlinked public command.
new_source source-brew
brew_root="$test_root/brew/Cellar/mole/1.55.0/libexec"
mkdir -p "$brew_root" "$test_root/brew/bin"
cp "$test_root/pristine/mole" "$brew_root/mole"
brew_original_hash="$(shasum -a 256 "$brew_root/mole")"
cp -R "$test_root/pristine/bin" "$brew_root/bin"
cp -R "$test_root/pristine/lib" "$brew_root/lib"
cp "$test_root/script/config/bin/analyze-go" "$brew_root/bin/analyze-go"
cp "$test_root/script/config/bin/status-go" "$brew_root/bin/status-go"
ln -s "$brew_root/mole" "$test_root/brew/bin/mole"
"$MOLE_ZH_HOME/manager" init "$test_root/brew/bin/mole" > "$test_root/brew-install.log" 2>&1
"$MOLE_ZH_HOME/bin/mo" --help > "$test_root/brew-help.txt"
grep -q '清理磁盘空间' "$test_root/brew-help.txt"
# A same-version Homebrew relink must follow the public symlink, not the old keg.
new_source source-brew-relink
new_brew_root="$test_root/brew/Cellar/mole/1.55.0_1/libexec"
mkdir -p "$new_brew_root"
cp "$test_root/pristine/mole" "$new_brew_root/mole"
cp -R "$test_root/pristine/bin" "$new_brew_root/bin"
cp -R "$test_root/pristine/lib" "$new_brew_root/lib"
cp "$test_root/script/config/bin/analyze-go" "$new_brew_root/bin/analyze-go"
cp "$test_root/script/config/bin/status-go" "$new_brew_root/bin/status-go"
ln -sfn "$new_brew_root/mole" "$test_root/brew/bin/mole"
"$MOLE_ZH_HOME/bin/mo" --help > "$test_root/brew-relink-help.txt" 2> "$test_root/brew-relink.log"
grep -q '清理磁盘空间' "$test_root/brew-relink-help.txt"
grep -q '已应用' "$test_root/brew-relink.log"
"$MOLE_ZH_HOME/bin/mole-zh" uninstall > /dev/null
[[ "$(shasum -a 256 "$brew_root/mole")" == "$brew_original_hash" ]]

# A malformed source build fails before modifying the installed official files.
new_source source-broken
printf '\nthis is invalid Go\n' >> "$MOLE_ZH_SOURCE/cmd/status/view.go"
brew_hash="$(shasum -a 256 "$new_brew_root/mole")"
if "$MOLE_ZH_HOME/manager" init "$test_root/brew/bin/mole" > "$test_root/broken.log" 2>&1; then
    echo "expected broken Go source to fail" >&2
    exit 1
fi
[[ "$(shasum -a 256 "$new_brew_root/mole")" == "$brew_hash" ]]
[[ ! -e "$MOLE_ZH_HOME/state.json" ]]

echo "mole-zh integration tests passed"
