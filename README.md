# Mole 中文插件

这是一个**独立于 Mole 的汉化插件**。先安装 [官方 Mole](https://github.com/tw93/Mole)，再安装本插件。Mole 目前没有原生插件接口；本项目会备份并修改已安装程序中的界面文案，不维护或分发另一份 Mole 源码。

支持官方脚本安装和 Homebrew 安装，适配 Mole V1 系列稳定版。已有翻译能在小版本更新后继续匹配；官方新增或改写的文字可能暂时显示英文。V2、非稳定版和无法识别的安装会保持官方原样并给出原因。

## 安装

需要 macOS、Git 和 Go。插件不会自行安装 Go。请先确认 `mole --version` 能运行，再执行：

```bash
git clone https://github.com/oujnit/mole-zh.git
cd mole-zh
./install.sh
```

安装脚本会显示一条 `export PATH=...` 命令。运行该命令可在当前终端立即启用；新终端会自动启用。`mo`、`mole` 保持原有用法。若电脑有多个 Mole 安装，可用 `./install.sh --target /完整路径/mole` 指定当前要汉化的官方安装。

插件只获取与已安装版本相同的官方源码，用于核对原文和构建中文 `analyze`、`status` 程序。原版文件备份在 `~/.local/share/mole-zh/backups/`。需要写入受保护安装目录时，安装命令会请求管理员权限；日常启动不会弹出提权提示。

## 更新和状态

继续使用 `mo update` 更新官方 Mole。直接运行 `brew upgrade mole` 或官方安装脚本也可以：下次通过插件入口启动 `mo` 时，会检测官方文件变化并尝试恢复汉化。安装结构、源码或编译检查失败时，插件不使用有问题的补丁，并让官方版本继续运行。

```bash
mole-zh status          # 版本、已应用翻译及未匹配的旧规则
mole-zh status --all    # 查看全部未匹配规则
mole-zh apply           # 手动重试汉化，必要时请求管理员权限
mole-zh uninstall       # 仅卸载汉化插件，恢复对应官方文件
```

直接用绝对路径调用官方 `mole` 会绕过插件入口的更新检测；已应用的汉化仍会显示，直到官方更新覆盖这些文件。插件不改命令参数或 JSON 等机器可读字段。`mo remove` 仍执行 Mole 自己的卸载流程。

## 限制与来源

- V1.55.0 是首版翻译规则的基线。状态中的“未匹配旧规则”能指出原文变化；新增英文文案仍需要人工补译，不能保证未来版本百分之百中文。
- Homebrew 或官方安装脚本会覆盖已汉化文件。插件入口会在下次启动时重新应用；若目录不可写且没有已缓存的管理员授权，请运行 `mole-zh apply`。
- 翻译基于 [tw93/Mole](https://github.com/tw93/Mole) 的界面文本。本项目遵循 GPL-3.0-or-later，详见 [LICENSE](LICENSE)。
