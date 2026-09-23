#!/bin/bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
    echo "mole-zh 目前仅支持 macOS。" >&2
    exit 1
fi

for tool in go git; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "缺少 $tool。请先安装，再重新运行插件安装脚本。" >&2
        exit 1
    fi
done

target=""
if [[ $# -eq 2 && "$1" == "--target" ]]; then
    target="$2"
elif [[ $# -ne 0 ]]; then
    echo "用法：./install.sh [--target /absolute/path/to/mole]" >&2
    exit 2
fi

plugin_home="${MOLE_ZH_HOME:-$HOME/.local/share/mole-zh}"
updating=false
if [[ -f "$plugin_home/state.json" ]]; then
    updating=true
    if [[ -n "$target" ]]; then
        echo "插件已安装；更新时无需 --target。如需切换官方安装，请先卸载插件。" >&2
        exit 1
    fi
    if [[ ! -x "$plugin_home/manager" ]]; then
        echo "插件状态存在，但管理程序丢失；请先修复本地安装。" >&2
        exit 1
    fi
else
    if [[ -z "$target" ]]; then
        target="$(command -v mole || true)"
    fi
    if [[ -z "$target" || ! -f "$target" ]]; then
        echo "没有找到官方 Mole。请先安装官方版，再安装汉化插件。" >&2
        exit 1
    fi
fi

mkdir -p "$plugin_home"
chmod 700 "$plugin_home"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmp_binary="$(mktemp "$plugin_home/.manager.XXXXXX")"
previous_binary=""
trap 'rm -f "$tmp_binary" "$previous_binary"' EXIT
(cd "$script_dir" && go build -o "$tmp_binary" ./cmd/mole-zh)
chmod 755 "$tmp_binary"
if [[ "$updating" == "true" ]]; then
    previous_binary="$(mktemp "$plugin_home/.manager-previous.XXXXXX")"
    cp -p "$plugin_home/manager" "$previous_binary"
    mv -f "$tmp_binary" "$plugin_home/manager"
    if ! "$plugin_home/manager" apply; then
        mv -f "$previous_binary" "$plugin_home/manager"
        echo "翻译规则更新失败，已恢复旧版插件。" >&2
        exit 1
    fi
    echo "汉化插件已更新。"
    exit 0
fi
mv -f "$tmp_binary" "$plugin_home/manager"
if ! "$plugin_home/manager" init "$target"; then
    rm -f "$plugin_home/manager"
    exit 1
fi
