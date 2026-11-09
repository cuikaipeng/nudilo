#!/usr/bin/env python3
"""Generate release notes from the same version and asset names as packaging."""
import os
from release import config, asset_names

cfg = config(); version = cfg['version']
repo = os.environ.get('GITHUB_REPOSITORY', 'cuikaipeng/nudilo')
base = f'https://github.com/{repo}/releases/download/v{version}'
print(f'''Nudilo {version} — A little nudge from your AI.

**English**

A native Codex and Claude Code notification companion for macOS, Windows and Linux.

- Coral theme, light/dark/system appearance.
- English and Simplified Chinese, following the system by default.
- DingTalk, Feishu, Lark, Telegram and Slack notifications.
- Signed background updates and manual update checks; restart when ready.

**Downloads / 下载**

| Platform / 系统 | Package / 安装包 |
| --- | --- |''')
for label,target in [('macOS · Intel + Apple Silicon','darwin/universal'),('Windows · x64','windows/amd64'),('Linux · x64','linux/amd64'),('Linux · ARM64','linux/arm64')]:
    links = [f'[{name}]({base}/{name})' for name in asset_names(version,target) if name!='install.sh']
    print('| '+label+' | '+' · '.join(links)+' |')
print(f'''
Linux tarball installs also need [install.sh]({base}/install.sh).
Verify downloads with [SHA256SUMS]({base}/SHA256SUMS).
The `update-*.json` manifests and platform `.tar.gz` archives are used by the in-app updater.

macOS builds are ad-hoc signed and not notarized. Windows builds are unsigned.
Review the changes and complete installation checks before publishing this draft.

---

**简体中文**

支持 macOS、Windows 和 Linux 的原生 Codex 与 Claude Code 通知助手。

- 珊瑚色主题，浅色、深色和跟随系统外观。
- 英文与简体中文，默认跟随系统语言。
- 支持钉钉、飞书、Lark、Telegram 和 Slack。
- 支持签名后台更新及手动检查，由你选择何时重启。

Linux 压缩包安装还需下载 install.sh；SHA256SUMS 用于校验下载文件。
update-*.json 与各平台 tar.gz 是应用内更新所需文件。
macOS 包采用临时签名，未经 Apple 公证；Windows 包未签名。
发布此草稿前，请核对更新说明并完成各系统的安装检查。
''')
