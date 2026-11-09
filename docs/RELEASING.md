# Builds and releases

Nudilo follows Kivrix's approach: reproducible version checks, isolated
packaging, verified installers, checksums and staged GitHub releases.
It uses its own repository, branding and notification helper. No Kivrix update
keys or update feeds are reused. Native in-app updates use Nudilo's own
Ed25519 signing key; see [automatic updates and key setup](UPDATES.md)
([中文](UPDATES.zh-CN.md)).

## Local packaging

Requirements: Go 1.27.3, Python 3.9+ and the platform's packaging tools.
MyGo is pinned to v0.7.4. DMGs require macOS. Windows installers require NSIS
3.13 (`makensis`), matching the reviewed source and notices. On Windows the
packaging script uses an installed matching version or downloads the pinned,
checksum-verified compiler into its temporary directory. Cross-build hosts
must have NSIS 3.13 installed.
Linux packages do not require a browser runtime.
Packaging requires the matching update signing key, independently of optional
OS certificates. Local builds read the private key outside the repository;
CI uses the `MYGO_UPDATER_PRIVATE_KEY` repository secret. See [key setup](UPDATES.md#release-signing-setup).

```sh
make package
PLATFORM=darwin/universal,windows/amd64,linux/amd64,linux/arm64 make package
python3 scripts/release.py verify dist/release
```

Windows: `./scripts/build.ps1 -Platform windows/amd64`.

Packaging copies source inputs into a temporary directory, prepares approved
brand assets and third-party notices, compiles the event helper for every
architecture, derives macOS language metadata from the catalogs, and invokes
MyGo. It does not modify the working tree's configuration or signing settings.

Checks cover app/helper architectures, macOS signatures and languages, DEB
metadata, native Linux dependencies, and the exact packaged brand/license
resources. Windows version metadata is checked on a Windows host. Local
cross-compilation does not replace installation testing on the target OS.

Before packaging, `make licenses` (also part of `make check`) must pass. The
package command runs the same read-only check itself. It verifies the reviewed
Go/dependency versions, license and asset hashes, full notice text and covered
source copies. Every package includes `THIRD_PARTY_NOTICES.txt` plus the original
Public Suffix List and NSIS source in `third-party-sources/`; resource checks
compare the packaged files byte-for-byte. Windows checks inspect the installer
staging files; install the final executable on Windows as part of release QA.
See [licensing](LICENSING.md) and [the update workflow](../third_party/README.md)
before upgrading dependencies or changing installer versions.

Verified download assets are staged in `dist/release/`:

- `nudilo-X.Y.Z-macOS.dmg` — Intel and Apple Silicon.
- `nudilo-X.Y.Z-windows-amd64.exe` — Windows x64 installer.
- `nudilo-X.Y.Z-linux-{amd64,arm64}.deb` and `.tar.gz`.
- `install.sh` — download beside a Linux tarball; run `sh install.sh`.
- `SHA256SUMS` — verify downloads with `shasum -a 256 -c SHA256SUMS` or `sha256sum -c SHA256SUMS`.
- `nudilo-X.Y.Z-<platform>-<architecture>.tar.gz` and `update-<platform>-<architecture>.json` — signed native update archives and manifests. Linux reuses its installation tarball.

Full app bundles remain in their `dist/<platform>-<arch>/` directories.

## Version and GitHub workflow

`dev` is the default development branch. Submit ordinary changes to `dev`;
merge a release PR from `dev` into `main` when the version is ready to ship.
Every push to `main`, including a direct push, starts the release pipeline,
so reserve `main` for releases and use a PR for review.

Before opening the release PR, update `mygo.json` and
`internal/buildinfo/version.go` together to a version higher than every published
stable release (for example, `0.1.0` → `0.1.1`). Run
`python3 scripts/release.py check --tag vX.Y.Z`. The workflow reads this version;
it does not choose a version or increment one automatically. A PR targeting
`main` also checks GitHub for a published version or an already used tag.

| Trigger | Checks | Installers | Release |
| --- | --- | --- | --- |
| Push to `dev` or pull request | Go, UI, i18n, release-script tests, vet, race and five architecture builds | No | No |
| Manual workflow | Same | All desktop targets | Artifacts only |
| Merge/push to `main` | Same, plus release-version guard | All desktop targets | Automatically tag and publish |
| Push a tag | No workflow | No | No |

Only the release job has repository write permission. Packaging jobs must all
succeed before asset verification and checksum generation. The job creates
`vX.Y.Z` at **the exact `main` commit that triggered the workflow**, even though
the repository's default branch is `dev`. It uploads the bilingual notes and all
assets to a draft, verifies the uploaded filenames, sizes and SHA-256 digests,
then automatically publishes a stable release and marks it as latest. No manual
tag or publication step is needed. New pushes do not cancel a running `main`
release; ordinary `dev`/PR runs cancel their superseded checks.

Publication guards run before building, before uploading and before publishing.
Published versions are never overwritten, older versions are rejected, and
existing tags are never moved. An interrupted run may leave a tag or draft;
rerun the failed workflow for **the same commit** to resume it. If a code fix is
needed after a tag was created, use a new version and another release PR.
Incomplete or mismatched uploads remain drafts and are not offered to app users.

Run the manual workflow on `dev` for installer preflight when needed; it never
creates a tag or publishes, even when run on `main`. Before merging the release
PR, review the bilingual notes generated by `scripts/release_notes.py` and test
installation/launch on the target systems. Installer artifacts expire after
seven days. Diagnostic UI previews upload only after a failure or during a manual
workflow, expire after three days, and do not fail the job if upload is unavailable
(for example, when the account's artifact storage quota is full). Installer uploads
remain required; a release cannot proceed without them.

## Signing

Local and default CI builds need no certificates: macOS uses ad-hoc signing
and is not notarized; Windows installers are unsigned. Their first launch is
subject to the OS and organization's security policy.

Optional signing uses credentials provided outside the repository:

- macOS: `NUDILO_MACOS_SIGNING_IDENTITY` and `NUDILO_NOTARY_PROFILE` (an existing `notarytool` keychain profile).
- Windows: `NUDILO_WINDOWS_CERTIFICATE` and MyGo's `MYGO_WINDOWS_CERTIFICATE_PASSWORD`.

Run `make package ARGS=--signed` or `./scripts/build.ps1 -Signed` to require
signing. Missing configuration fails before building. Signed Windows packages
must be verified on Windows. The default GitHub workflow intentionally produces
unsigned/ad-hoc packages; wire signing secrets and certificate import steps in
your own repository when ready.
This refers to OS code-signing certificates. Update archives always require
Nudilo's Ed25519 signature, and release verification cryptographically checks it.

## Validation

```sh
make check
make race
make test-native
NUDILO_NATIVE_CODEX=1 make test-native
CALLBELL_CODEX_TEST_BINARY=/absolute/path/to/codex go test -v ./internal/codex -run TestRealCodex
```

Native smoke runs on macOS with isolated data and no real notification
credentials. The default checks launch, channel setup, live language/theme
changes, persistence and close-to-tray. The optional Codex check requires an
installed Codex executable and uses a separate Codex home to verify registration,
helper fixtures, existing-chat monitoring, privacy and background processing.
The `TestRealCodex` tests additionally exercise an actual Codex turn using a
loopback model fixture, starting a chat before installing Nudilo, verifying
Stop execution in a new chat, and checking delivery/deduplication with a fake
IM sender. They need no API key and send no external messages. Release builds
omit the instrumented automation entry point.

Each packaging job also executes the Codex hook wrapper with a temporary data
folder, Unicode paths and a UTF-8 event. This checks the actual Windows
PowerShell and Unix shell paths without launching Codex or contacting an IM.

## 中文摘要

默认使用临时签名的 macOS 包与未签名 Windows 包，不需要证书。
应用内更新包仍需独立的 Ed25519 签名；首次发版前需配置
`MYGO_UPDATER_PRIVATE_KEY` 仓库 Secret，详见[自动更新说明](UPDATES.zh-CN.md)。
`dev` 是默认开发分支，日常提交及 PR 只运行检查，不打包或发版。准备发版时，在
`dev` 同时修改 `mygo.json` 和 `internal/buildinfo/version.go`，版本号必须高于所有
已发布的稳定版，例如 `0.1.0` → `0.1.1`。工作流不会自动决定或递增版本号。
通过 `dev` → `main` 的 PR 发布，合并前也会检查版本是否已经发布、标签是否被占用。
直接推送 `main` 同样会触发发版，因此应保留 `main` 用于正式发布，并使用 PR 审查。

合入 `main` 后自动检查、打包所有平台、验签并生成校验和。`vX.Y.Z` 标签固定指向
本次触发工作流的 `main` 提交，不受默认分支 `dev` 影响。所有附件先上传到草稿，
确认文件名、大小与 SHA-256 全部一致后，自动公开为最新稳定版，无需手动打标签或发布。
新提交不会中断正在进行的 `main` 发版；`dev` 和 PR 的过时检查可以被新提交取消。

已公开版本禁止覆盖，不允许版本倒退或移动已有标签。中断后可以重跑同一提交的工作流；
若已创建标签后还需要修改代码，应提高版本号后重新提交发布 PR。上传不完整或校验失败
时保持草稿，应用内不会提示该版本。单独推送标签不会触发工作流。

手动运行工作流可打包预检，即使选择 `main` 也不创建标签或发版。合并发布 PR 前，
核对 `scripts/release_notes.py` 生成的双语说明，并完成各系统安装与启动检查。
安装包附件保留七天。界面预览仅在检查失败或手动运行时上传，保留三天；
预览上传失败（例如账户附件存储配额已满）不会影响检查结果，安装包上传仍是必需步骤。
`make check` 和打包前均检查许可清单；所有安装包附带完整许可、Public Suffix List
原始数据和 NSIS 源码。Windows 安装器固定为与源码对应的 NSIS 3.13，资源核对不能
替代最终的 Windows 安装验证。依赖升级流程见[中文许可检查](LICENSING.zh-CN.md)。
