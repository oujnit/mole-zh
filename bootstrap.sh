#!/bin/bash
# Remote entrypoint: clone into a disposable directory and run the installer.
set -euo pipefail

main() {
    if [[ "$(uname -s)" != "Darwin" ]]; then
        echo "mole-zh 目前仅支持 macOS。" >&2
        return 1
    fi
    local tool
    for tool in git go; do
        if ! command -v "$tool" >/dev/null 2>&1; then
            echo "缺少 ${tool}。请先安装，再重新运行一键汉化命令。" >&2
            if [[ "$tool" == "go" ]]; then
                echo "已安装 Homebrew 的用户可运行：brew install go" >&2
            fi
            return 1
        fi
    done

    local work_dir
    work_dir="$(mktemp -d "${TMPDIR:-/tmp}/mole-zh-install.XXXXXX")"
    # The directory is private, newly created by mktemp, and contains only
    # this download. Keep the trap active when the installer returns an error.
    trap 'rm -rf -- "$work_dir"' EXIT
    echo "正在下载 Mole 汉化插件…"
    git clone --quiet --depth=1 https://github.com/oujnit/mole-zh.git "$work_dir/source"
    /bin/bash "$work_dir/source/install.sh" "$@"
    rm -rf -- "$work_dir"
    trap - EXIT
}

main "$@"
