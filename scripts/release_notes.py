#!/usr/bin/env python3
"""Generate matching English and Chinese notes from the packaging metadata."""
import os
from release import config, asset_names

PLATFORMS = (
    ('macOS · Intel + Apple Silicon', 'darwin/universal'),
    ('Windows · x64', 'windows/amd64'),
    ('Linux · x64', 'linux/amd64'),
    ('Linux · ARM64', 'linux/arm64'),
)
MACOS_COMMAND = 'xattr -dr com.apple.quarantine "/Applications/Nudilo.app"'
LANGUAGES = (
    {
        'name': 'English',
        'intro': 'A native Codex and Claude Code notification companion for macOS, Windows and Linux.',
        'features': (
            'Coral theme with light, dark and system appearance modes.',
            'English and Simplified Chinese, following the system language by default.',
            'Notifications through DingTalk, Feishu, Lark, Telegram and Slack.',
            'Signed background updates and manual update checks; you choose when to restart.',
        ),
        'downloads': 'Downloads',
        'platform': 'Platform',
        'package': 'Package',
        'linux': 'Linux tarball installs also need [install.sh]({base}/install.sh).',
        'checksums': 'Verify downloaded files with [SHA256SUMS]({base}/SHA256SUMS).',
        'updates': 'The `update-*.json` manifests and platform `.tar.gz` archives are used by the in-app updater.',
        'installation': 'Installation',
        'signing': 'macOS builds are ad-hoc signed and not notarized by Apple. Windows builds are unsigned.',
        'guide': 'See the [installation and first-launch guide]({source}/README.md#installation).',
        'macos': 'macOS first launch',
        'macos_instructions': "Move **Nudilo.app** to **Applications**. If macOS blocks a trusted download because it is not notarized, follow the guide above or remove this app's download quarantine attribute in Terminal:",
        'reopen': 'Then open Nudilo again from Applications.',
    },
    {
        'name': '简体中文',
        'intro': '支持 macOS、Windows 和 Linux 的原生 Codex 与 Claude Code 通知助手。',
        'features': (
            '珊瑚色主题，支持浅色、深色和跟随系统外观。',
            '支持英文与简体中文，默认跟随系统语言。',
            '通过钉钉、飞书、Lark、Telegram 和 Slack 发送通知。',
            '支持签名后台更新及手动检查，由你选择何时重启。',
        ),
        'downloads': '下载',
        'platform': '系统',
        'package': '安装包',
        'linux': '使用 Linux 压缩包安装时，还需下载 [install.sh]({base}/install.sh)。',
        'checksums': '使用 [SHA256SUMS]({base}/SHA256SUMS) 校验下载文件。',
        'updates': '`update-*.json` 更新清单与各平台的 `.tar.gz` 压缩包供应用内更新使用。',
        'installation': '安装',
        'signing': 'macOS 包采用 ad-hoc 临时签名，未经 Apple 公证；Windows 包未签名。',
        'guide': '请参阅[安装及首次打开说明]({source}/README.zh-CN.md#安装)。',
        'macos': 'macOS 首次打开',
        'macos_instructions': '先将 **Nudilo.app** 拖入**应用程序**。如果可信下载因未经公证被 macOS 拦截，请参考上方说明，或在终端执行以下命令，移除该应用的下载隔离标记：',
        'reopen': '然后从“应用程序”重新打开 Nudilo。',
    },
)


def release_notes(cfg, repo):
    version = cfg['version']
    urls = {
        'base': f'https://github.com/{repo}/releases/download/v{version}',
        'source': f'https://github.com/{repo}/blob/v{version}',
    }
    sections = []
    for language in LANGUAGES:
        lines = [f"## {language['name']}", '', language['intro'], '']
        lines.extend('- ' + feature for feature in language['features'])
        lines.extend([
            '', f"### {language['downloads']}", '',
            f"| {language['platform']} | {language['package']} |", '| --- | --- |',
        ])
        for label, target in PLATFORMS:
            links = [f'[{name}]({urls["base"]}/{name})'
                     for name in asset_names(version, target) if name != 'install.sh']
            lines.append('| ' + label + ' | ' + ' · '.join(links) + ' |')
        for key in ('linux', 'checksums', 'updates'):
            lines.extend(['', language[key].format(**urls)])
        lines.extend([
            '', f"### {language['installation']}", '', language['signing'],
            '', language['guide'].format(**urls),
            '', f"#### {language['macos']}", '', language['macos_instructions'],
            '', '```sh', MACOS_COMMAND, '```', '', language['reopen'],
        ])
        sections.append('\n'.join(lines))
    return f'Nudilo {version} — A little nudge from your AI.\n\n' + '\n\n---\n\n'.join(sections) + '\n'


if __name__ == '__main__':
    cfg = config()
    print(release_notes(cfg, os.environ.get('GITHUB_REPOSITORY', cfg['updates']['github'])), end='')
