# Contributing

By intentionally submitting a contribution for inclusion in Nudilo, you offer
it under the project's [MIT license](LICENSE), unless you explicitly identify
separately licensed third-party material. Contribute only material you have the
right to provide. Retain upstream copyright notices and licenses, record the
source and changes, and do not relicense others' work as Nudilo's own.
No separate CLA or mandatory DCO sign-off is required.

Nudilo is a native Go/MyGo desktop app. Keep the UI native, retain existing
Codex/credential identifiers for upgrade compatibility, and never use real IM
credentials in tests.

`dev` is the default branch. Base development work on `dev` and target ordinary
PRs there; use Conventional Commits such as `fix(channels): handle expired tokens`.
Its pushes and PRs run checks only. To release, update both version files and
merge a `dev` → `main` PR: `main` runs the full checks/package pipeline and
automatically tags and publishes the new version. Review release notes and
installation tests before merging; do not push routine development to `main`.

```sh
make check
make race
make test-ui
```

Use semantic colors in `internal/desktop/theme.go` and preserve the approved
TOUCH assets. Check light/dark appearances and English/Chinese layouts. Keep
copy short; put setup instructions in the channel's expandable help. Add user
visible strings to both catalogs; see [translations](docs/I18N.md).

Keep the notification helper fast and offline. Do not approve Codex actions,
expose credential values in errors, or broaden the default notification payload.
Cover changes to retry behavior, deduplication, locale migration or credential
handling with behavioral tests.

Packaging and version changes follow [RELEASING](docs/RELEASING.md). Uploaded
assets are verified in a draft before automatic publication. Do not commit signing
material, local settings, generated installers or real notification history.

Dependency, icon and installer changes must follow the
[license review workflow](third_party/README.md). Run `make licenses` before
packaging; review upstream terms before refreshing the inventory.

## 贡献许可

有意提交给 Nudilo 的贡献默认使用项目 MIT 许可；另有许可的第三方材料需要明确标注。
请确保有权提交相关内容，保留上游版权及许可，并记录来源和改动。本项目不额外要求
CLA 或强制 DCO 签署。依赖、图标和安装器变更需要检查许可，详见
[中文许可说明](docs/LICENSING.zh-CN.md)。

默认开发分支是 `dev`，日常修改和 PR 均以它为基础，提交信息采用 Conventional Commits，
例如 `fix(channels): handle expired tokens`。日常提交只运行检查。发版时同时提高两个
版本文件中的版本号，审查双语发版说明并完成安装测试后，再合并 `dev` → `main` 的 PR；
`main` 自动完成检查、打包、打标签和正式发布。不要向 `main` 提交日常开发变更。
