# Mole 中文插件

给本机已安装的 [官方 Mole](https://github.com/tw93/Mole) 加上中文界面。**这是汉化插件，需要先有官方 Mole**；它会备份并汉化现有安装中的文件。

## 一键汉化本机 Mole

**已经安装官方 Mole？复制这一条命令，即可汉化现有安装，无需手动克隆仓库或进入目录：**

```bash
curl -fsSL https://raw.githubusercontent.com/oujnit/mole-zh/main/bootstrap.sh | bash
```

需要 **macOS、Git 和 Go 1.23+**，并确保 `mole --version` 能正常运行。缺少 Go 且已安装 Homebrew 时，先运行 `brew install go`。本命令不会安装 Mole 或自动安装这些依赖。

完成后，**重新打开终端**，继续使用 `mo` 或 `mole`，界面即为中文。已有终端也可按安装输出中的 `export PATH=...` 命令立即启用。运行 `mole-zh status` 可检查汉化状态。

一键入口会临时下载插件并自动清理下载目录；重复执行同一条命令即可更新插件与翻译规则。原版文件备份在 `~/.local/share/mole-zh/backups/`。需要写入受保护的安装目录时，安装过程会请求管理员权限。

<details>
<summary>手动安装／指定 Mole 路径</summary>

```bash
git clone https://github.com/oujnit/mole-zh.git
cd mole-zh
./install.sh
```

若电脑有多个 Mole 安装，可用 `./install.sh --target /完整路径/mole` 指定要汉化的安装。已有插件需要切换目标时，先运行 `mole-zh uninstall`。

插件会获取与已安装版本相同的官方源码，核对原文并构建中文 `analyze`、`status` 程序。

</details>

## 更新和状态

继续使用 `mo update` 更新官方 Mole。直接运行 `brew upgrade mole` 或官方安装脚本也可以：下次通过插件入口启动 `mo` 时，会检测官方文件变化并尝试恢复汉化。安装结构、源码或编译检查失败时，插件不使用有问题的补丁，并让官方版本继续运行。

更新汉化插件：**重新执行上面的一键命令即可**。手动安装的用户也可在仓库目录运行 `git pull && ./install.sh`。更新失败时会恢复旧版管理程序。

```bash
mole-zh status          # 版本、已应用翻译及未匹配的旧规则
mole-zh status --all    # 查看全部未匹配规则
mole-zh apply           # 手动重试汉化，必要时请求管理员权限
mole-zh uninstall       # 仅卸载汉化插件，恢复对应官方文件
```

直接用绝对路径调用官方 `mole` 会绕过插件入口的更新检测；已应用的汉化仍会显示，直到官方更新覆盖这些文件。插件不改命令参数或 JSON 等机器可读字段。`mo remove` 仍执行 Mole 自己的卸载流程。

## 限制与来源

支持官方脚本安装和 Homebrew 安装，适配 Mole V1 系列稳定版。V2、非稳定版和无法识别的安装不会应用汉化。Mole 目前没有原生插件接口，本插件通过修改已安装文件实现汉化。

- V1.62.0 是当前翻译规则的基线（首版基线为 V1.55.0）。状态中的“未匹配旧规则”能指出原文变化；新增英文文案仍需要人工补译，不能保证未来版本百分之百中文。
- Homebrew 或官方安装脚本会覆盖已汉化文件。插件入口会在下次启动时重新应用；若目录不可写且没有已缓存的管理员授权，请运行 `mole-zh apply`。
- 翻译基于 [tw93/Mole](https://github.com/tw93/Mole) 的界面文本。本项目遵循 MIT 许可证，详见 [LICENSE](LICENSE)。
