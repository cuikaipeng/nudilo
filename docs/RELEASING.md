# Builds and releases

Nudilo follows Kivrix's approach: reproducible version checks, isolated
packaging, verified installers, checksums and draft GitHub releases.
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

Update `mygo.json` and `internal/buildinfo/version.go` together.
`python3 scripts/release.py check --tag vX.Y.Z` must pass before tagging.

| Trigger | Checks | Installers | Release |
| --- | --- | --- | --- |
| Push to main/dev or pull request | Go, UI, i18n, release-script tests, vet, race and five architecture builds | No | No |
| Manual workflow | Same | All desktop targets | Artifacts only |
| Push matching `vX.Y.Z` tag | Same | All desktop targets | Draft only |

Only the release job has repository write permission. Packaging jobs must all
succeed before asset verification and checksum generation. A publication guard
runs both before building and before uploading: already published versions are
never overwritten; draft retries are allowed. Artifacts expire after seven days.

Review the bilingual draft notes, install and launch on each target, and
publish the draft manually when ready. Editing workflow files locally does not
publish a release or set repository secrets.

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
版本号必须在 `mygo.json` 和 `internal/buildinfo/version.go` 中一致；版本标签
触发三平台构建、产物检查和校验和生成，最终只创建草稿，不自动公开发布。
正式发布前核对双语说明并完成各系统安装检查。已公开版本禁止覆盖。
`make check` 和打包前均检查许可清单；所有安装包附带完整许可、Public Suffix List
原始数据和 NSIS 源码。Windows 安装器固定为与源码对应的 NSIS 3.13，资源核对不能
替代最终的 Windows 安装验证。依赖升级流程见[中文许可检查](LICENSING.zh-CN.md)。
