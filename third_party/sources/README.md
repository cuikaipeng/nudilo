# Bundled source provenance

These are complete, unmodified upstream source inputs for components with
source-availability conditions. They are retained in this repository and copied
byte-for-byte into every installer under `third-party-sources/`. The common
resource bundle intentionally includes them on all platforms, although the
current uses described below are Windows-specific.

## Public Suffix List

- File: `public_suffix_list.dat`
- License: [MPL-2.0](../licenses/public-suffix-list.txt)
- Revision: `d6c92f1bbb7433e5db7b8405c25d4035fb8ff376`
- Source: [original data](https://github.com/publicsuffix/list/blob/d6c92f1bbb7433e5db7b8405c25d4035fb8ff376/public_suffix_list.dat)
- SHA-256: `a5638281157e8c902b127a5376f9ea2d024bf2ee11524133a6a76cd1d14ee7be`

`golang.org/x/net/publicsuffix` v0.62.0 embeds tables generated from this exact
list; its `table.go` records the revision. The Go representation and generator
are available at the [fixed Go module version](https://go.googlesource.com/net/+/refs/tags/v0.62.0/publicsuffix/).
Nudilo makes no changes to the list or generator.

## NSIS 3.13

- File: `nsis-3.13-source.tar.gz`
- License: [upstream COPYING](../licenses/nsis.txt), including Zlib, bzip2 and CPL-1.0
- Tag / revision: `v313` / `a4ba28e60bd9e7c271cd2bfc1558889ce3355653`
- Source: [complete upstream source tree](https://github.com/NSIS-Dev/nsis/tree/a4ba28e60bd9e7c271cd2bfc1558889ce3355653)
- Archive: [GitHub archive at that commit](https://api.github.com/repos/NSIS-Dev/nsis/tarball/a4ba28e60bd9e7c271cd2bfc1558889ce3355653)
- SHA-256 of the retained archive: `660c4f684ee4b5879d86e38cc036b7799fc8c077e3691829201802b21a854724`

The archive retains upstream files and notices, including the LZMA decoder
and build scripts. MyGo's Windows installer uses `/SOLID lzma` compression.
The installer engine is separate from the installed MIT-licensed Nudilo app.
No NSIS source modifications are made by Nudilo.

The Windows compiler ZIP is separately pinned in
[components.json](../components.json). A freshly regenerated GitHub tar archive
can differ in compression metadata; verify against the retained archive hash,
not a guessed hash for another packaging of the same source revision.

## 中文说明

这里保留完整、未修改的上游源码：Public Suffix List 为实际嵌入数据对应的原始列表，
NSIS 为 3.13 的完整源码压缩包。每个安装包都会在 `third-party-sources/` 中附带相同文件；
为统一资源清单，目前所有平台都携带，尽管上述用途在 Windows 上。
文件版本、下载来源及 SHA-256 如上。它们适用各自的原始许可，不会被项目的 MIT 覆盖。
升级依赖时应一起更新对应源码、许可和登记，不能只修改版本号或哈希。
