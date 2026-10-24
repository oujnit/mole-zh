#!/bin/bash
# Exercise the documented piped entrypoint without installing anything locally.
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/mole-zh-bootstrap-test.XXXXXX")"
trap 'rm -rf -- "$test_root"' EXIT
mkdir -p "$test_root/bin" "$test_root/tmp"
export TMPDIR="$test_root/tmp"
export BOOTSTRAP_TRACE="$test_root/trace"
export PATH="$test_root/bin:/usr/bin:/bin"

cat > "$test_root/bin/uname" <<'STUB'
#!/bin/bash
printf '%s\n' "${BOOTSTRAP_OS:-Darwin}"
STUB
cat > "$test_root/bin/go" <<'STUB'
#!/bin/bash
exit 0
STUB
cat > "$test_root/bin/git" <<'STUB'
#!/bin/bash
set -eu
printf '%s\n' "$@" > "$BOOTSTRAP_TRACE"
[[ "${BOOTSTRAP_CLONE_FAIL:-0}" == 0 ]] || exit 23
target="${!#}"
mkdir -p "$target"
cat > "$target/install.sh" <<'INSTALL'
#!/bin/bash
printf 'INSTALL:%s\n' "$@" >> "$BOOTSTRAP_TRACE"
exit "${BOOTSTRAP_INSTALL_FAIL:-0}"
INSTALL
STUB
chmod +x "$test_root/bin/"*

assert_clean() {
    [[ -z "$(ls -A "$test_root/tmp")" ]] || {
        echo 'Temporary download directory was not cleaned' >&2
        exit 1
    }
}

# Same stdin execution as curl | bash; arguments support an explicit target.
/bin/bash -s -- --target '/fixture/Mole install/mole' < "$repo_root/bootstrap.sh" > "$test_root/success.log"
grep -qx 'https://github.com/oujnit/mole-zh.git' "$BOOTSTRAP_TRACE"
grep -qx 'INSTALL:--target' "$BOOTSTRAP_TRACE"
grep -qx 'INSTALL:/fixture/Mole install/mole' "$BOOTSTRAP_TRACE"
assert_clean

# Both network errors and installer errors must propagate and clean downloads.
for failure in clone install; do
    rc=0
    if [[ "$failure" == clone ]]; then
        BOOTSTRAP_CLONE_FAIL=1 /bin/bash < "$repo_root/bootstrap.sh" > "$test_root/failure.log" 2>&1 || rc=$?
        [[ "$rc" == 23 ]]
    else
        BOOTSTRAP_INSTALL_FAIL=17 /bin/bash < "$repo_root/bootstrap.sh" > "$test_root/failure.log" 2>&1 || rc=$?
        [[ "$rc" == 17 ]]
    fi
    assert_clean
done

# Dependency and platform errors must occur before any download.
rm "$BOOTSTRAP_TRACE"
mv "$test_root/bin/go" "$test_root/bin/go-disabled"
if /bin/bash < "$repo_root/bootstrap.sh" > "$test_root/missing-go.log" 2>&1; then
    echo 'Missing Go was accepted' >&2
    exit 1
fi
grep -q 'brew install go' "$test_root/missing-go.log" || { cat "$test_root/missing-go.log"; exit 1; }
[[ ! -e "$BOOTSTRAP_TRACE" ]]
# The local installer must also report missing dependencies under macOS Bash 3.2.
if /bin/bash "$repo_root/install.sh" > "$test_root/local-missing-go.log" 2>&1; then
    echo 'Local installer accepted missing Go' >&2
    exit 1
fi
grep -q '缺少 go。' "$test_root/local-missing-go.log"
if BOOTSTRAP_OS=Linux /bin/bash < "$repo_root/bootstrap.sh" > "$test_root/platform.log" 2>&1; then
    echo 'Unsupported platform was accepted' >&2
    exit 1
fi
[[ ! -e "$BOOTSTRAP_TRACE" ]]
assert_clean
echo 'Bootstrap checks passed (isolated; local Mole untouched).'
