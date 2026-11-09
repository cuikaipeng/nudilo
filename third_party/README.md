# Third-party provenance and review

The root [MIT license](../LICENSE) covers Nudilo's own work. Third-party items
retain the licenses in this directory. See [the review](../docs/LICENSING.md)
([中文](../docs/LICENSING.zh-CN.md)) and [notice overview](../THIRD_PARTY_NOTICES.md).

- `components.json` is the manually reviewed registry: versions, license
  families, immutable source references, service-icon adaptations, source
  archives and the reviewed Windows installer compiler.
- `inventory.json` is the generated review snapshot: Go version, all selected
  modules, compiled modules across five targets, upstream notice hashes and
  repository file hashes. It contains no machine-specific paths.
- `licenses/` retains complete upstream texts. `licenses/modules/` and
  `licenses/go.txt` are generated; nested notices are retained even when a
  particular upstream subpackage is not shipped.
- `sources/` supplies source for covered data/installer components. These
  exact files are copied into `resources/third-party-sources/` for packaging.
- `resources/THIRD_PARTY_NOTICES.txt` combines the complete licenses,
  attribution and source-availability statements for redistribution.

## Check and update

Run `make licenses`. This is a read-only check; it does not silently accept
new dependencies or repair stale files. It is also part of `make check` and
runs before packaging.

When a dependency, Go version, icon or installer changes:

1. Inspect its upstream license, nested notices, data/assets and source
   obligations. A root MIT or Apache file may not cover every bundled item.
2. Update `components.json` with the reviewed versions and fixed sources.
   Copy any new full licenses and covered source into this directory. Preserve
   original copyright holders, years, conditions and disclaimers.
3. If updating NSIS, update its compiler version, ZIP checksum/URLs, matching
   complete source archive and COPYING text together. The current compiler
   is NSIS 3.13; Windows builds can download its pinned ZIP into the temporary
   build directory, while cross-build hosts need that version installed.
4. Remove obsolete generated module/source copies explicitly after review.
   Unexpected files in generated directories fail rather than being deleted
   automatically or accidentally included in installers.
5. Run `go run ./cmd/callbell-assets -refresh-licenses`, inspect the changes,
   then run `make check`. Build and verify affected platform packages.

Ordinary `go run ./cmd/callbell-assets` prepares approved brand assets and
recreates license copies only when their inputs match the reviewed inventory.
It does not update the inventory. The refresh command records a review; it is
not itself a legal review or a license detector for arbitrary source code.

The scan includes project tests and the pinned MyGo build tool as well as the
app/helper, for `darwin/amd64`, `darwin/arm64`, `windows/amd64`, `linux/amd64`
and `linux/arm64`, with CGO disabled. The current graph selects 21 external
modules, of which nine supply packages to these builds. Unused module metadata
is retained in the snapshot so graph changes remain visible. New build tags,
architectures, external binaries or embedded assets need explicit review.

Exact-byte files are protected from Git line-ending conversion by
`.gitattributes`. Do not edit upstream license text to make it fit the project's
MIT license. Keep source archives unmodified, with their internal notices.

## 中文说明

`components.json` 是人工审核的组件、版本、素材来源及安装器登记；
`inventory.json` 是对应的哈希与依赖快照。`make licenses` 只检查，不自动接受变化。
升级依赖或替换素材时，先检查上游许可及源码提供要求，再更新登记和原文文件，
运行 `go run ./cmd/callbell-assets -refresh-licenses`，检查差异并执行 `make check`。
NSIS 编译器、下载校验和、源码及许可必须一起更新。旧生成文件需要核对后明确删除。
第三方文件保留原始声明，不改写为 Nudilo 的 MIT。详细结果见[中文检查说明](../docs/LICENSING.zh-CN.md)。
