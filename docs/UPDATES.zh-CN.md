# 自动更新

[English](UPDATES.md)

Nudilo 使用 MyGo 原生签名更新机制，流程参考 Kivrix，使用独立的 Nudilo Ed25519
密钥和 `cuikaipeng/nudilo` 发布源，不复用 Kivrix 的密钥或更新地址。

## 使用方式

- 在「设置 → 更新」查看当前版本、自动更新开关、检查结果、下载进度及重启按钮。
- macOS 可在「Nudilo → 检查更新」手动检查，位于「关于 Nudilo」下方。
  Windows 和 Linux 的入口在「文件 → 检查更新」。菜单栏／托盘菜单也保留此入口。
- 默认开启自动更新，旧配置升级后同样开启。启动 30 秒后检查一次，之后运行期间每
  6 小时检查一次；重新打开自动更新开关也会立即检查。
- 发现新的稳定版本后自动下载并验证，当前 Nudilo 继续发送提醒，**不会自动重启**。
- 更新准备好后，点击「重启更新」。关闭窗口只是驻留托盘，不会重启；正常退出后重新
  启动也可启用准备好的版本。如果有未保存的渠道配置、通知文案或项目规则，重启前会提醒确认。
- 关闭自动更新后，可手动「检查更新 → 下载更新 → 重启更新」。关闭开关会阻止新的
  自动检查和下载，已经开始准备的更新会继续完成。
- 网络失败、下载或验证失败都会显示对应状态，可以重试；重复点击不会并行安装多份更新。
  界面跟随当前语言和深浅色模式。

更新只通过 HTTPS 和系统代理请求，禁止跳转到 HTTP。不发送 IM 凭据、通知记录、
项目名称或 GitHub 账号令牌。只接收公开的稳定版本，不接收草稿或预发布版本。
仓库私有、尚未发布或当前平台缺少更新清单时，会显示「暂时没有可用的公开更新」，
不会误报为最新版。

应用与通知助手一起更新。本地设置、加密凭据、历史记录和助手监听配置都位于安装目录
之外；重启后按原设置重新关联助手，使用更新后的通知程序，稳定的本地路径保持兼容。

## 各平台行为

| 安装方式 | 更新方式 |
| --- | --- |
| macOS 通用应用，位于可写的正常安装位置 | 验证并准备新版本，退出／重启时由 MyGo 替换应用包。 |
| Windows x64，当前用户可写的安装目录 | 验证并准备更新文件，重启后运行新版本。 |
| Linux x64 / ARM64，通过 tar.gz 安装在用户目录 | 验证后更新文件，重启生效。 |
| 只读位置、由系统管理的 `.deb` 等安装，或开发构建 | 仍能检查新版，提供官方下载入口，由用户或包管理器升级。 |

Nudilo 不会为替换系统管理的安装而请求管理员权限。此类安装应继续使用原有包管理器，
或下载安装新版。每个平台的更新包都包含通知助手、完整第三方许可及相关源码。

## 首次发布前的配置

`mygo.json` 中的 `updates.publicKey` 是更新公钥，打包时嵌入应用。更新包使用
Ed25519 对 SHA-256 摘要签名。它与 Apple 开发者证书、公证、Windows 代码签名是两回事；
即使使用临时签名的 macOS 包或未签名的 Windows 安装器，更新包也必须具有有效更新签名。

此电脑已经在仓库外生成独立密钥对：

```text
~/Library/Application Support/mygo/update-keys/nudilo/mygo-update.key
~/Library/Application Support/mygo/update-keys/nudilo/mygo-update.pub
```

私钥文件权限为 `0600`。请私下备份 `mygo-update.key`，后续发布继续使用同一密钥；
已安装版本只信任对应公钥。不要把私钥提交到 Git、粘贴到 issue 或在普通构建中重新生成。
打包前会校验私钥种子及其派生公钥，不会输出密钥内容。

本地构建按以下顺序读取签名私钥：

1. `MYGO_UPDATER_PRIVATE_KEY` 环境变量，内容为 base64 私钥。
2. `NUDILO_UPDATE_KEY_FILE` 环境变量，内容为私钥文件路径。
3. 系统配置目录下的 `mygo/update-keys/nudilo/mygo-update.key`。
   macOS 配置目录是 `~/Library/Application Support`，Windows 是 `%AppData%`，
   Linux 是 `$XDG_CONFIG_HOME` 或 `~/.config`。

在 GitHub 仓库的 **Settings → Secrets and variables → Actions** 中增加
**`MYGO_UPDATER_PRIVATE_KEY`**，值为该私钥文件的内容。工作流已经接好这个 Secret。
PR 检查不需要私钥；发版打包缺少匹配私钥时会明确失败。私钥不会包含在发布附件中。
完成配置后，推送到 `main` 即可自动发布签名更新。

Fork 项目应自行生成密钥并使用自己的发布仓库：运行
`go tool mygo keygen -o /私有目录`，修改 `updates.publicKey`、`updates.github`，
同时同步 `internal/buildinfo/version.go` 的 `Repository`。原版 Nudilo 不会信任 Fork 的新密钥。

## 发布更新

1. 配置一次 GitHub Secret，并备份私钥。
2. 在 `dev` 开发，同时提高 `mygo.json` 和 `internal/buildinfo/version.go` 的稳定版本号。
3. 运行 `make check`。可以在 `dev` 手动运行 Actions 打包所有平台预检，不会发版；
   在目标系统测试最终安装包。本地打包使用相同流程，不替换正在运行的应用。
4. 核对双语发版说明，再合并 `dev` → `main` 的发布 PR。CI 检查版本、构建所有平台并验签，
   将 `vX.Y.Z` 标签固定到本次 `main` 提交，再把全部附件上传到草稿。核对上传文件的大小和
   SHA-256 后，自动公开为最新稳定版，无需手动打标签或发布。
   仓库及 Release 必须公开可下载，应用内的匿名更新请求才能访问。
5. 所有平台的安装器、签名压缩包和更新清单会一起发布。清单名固定为
   `update-darwin-universal.json`、`update-windows-amd64.json`、
   `update-linux-amd64.json`、`update-linux-arm64.json`。
   更新压缩包名为 `nudilo-X.Y.Z-<系统>-<架构>.tar.gz`。

发版流程会实际验签，检查版本、URL、大小，并核对更新压缩包与安装器中的应用／助手字节，
以及许可和源码资源。遗漏或篡改会阻止发版。`SHA256SUMS` 覆盖安装器及所有更新文件。
当前关闭增量更新，每次使用完整签名包。
日常 `dev` 提交和 PR 只运行检查。已公开版本不能覆盖，也不能被更旧的版本替代；
上传不完整时保持草稿。详见[发布流程和失败重试](RELEASING.md#中文摘要)。

初始 0.1.0 包用于建立公钥和更新源，只有**更高版本**才会提示升级。尚无公开 Release 时，
不能验证真实线上升级链路；本地测试覆盖状态切换、安全请求、签名验证、产物内容及中英文
界面，不会更新真实安装。

框架说明：[MyGo v0.7.4 自动更新文档](https://github.com/egoist/mygo/blob/v0.7.4/docs/updates.md)。
