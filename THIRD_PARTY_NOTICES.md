# Third-party notices

Nudilo's own code is licensed under [MIT](LICENSE). The MIT grant does not
replace the licenses of dependencies, data, reference code or service artwork.
See the [licensing review](docs/LICENSING.md) ([中文](docs/LICENSING.zh-CN.md))
for scope, findings and maintenance instructions.

The complete redistributable texts and source-availability statements are in
[resources/THIRD_PARTY_NOTICES.txt](resources/THIRD_PARTY_NOTICES.txt). Every
installer includes this file and the source files in
[resources/third-party-sources](resources/third-party-sources).

## Go dependencies

The review covers the app, notification helper, project tests and pinned MyGo
build tool on macOS Intel/Apple Silicon, Windows x64 and Linux x64/ARM64.
The current compiled graph uses the following nine external modules. Full
module archives can contain extra packages; preserving their nested notices
does not mean all those packages are linked into Nudilo.

| Module | Version | License / additional notices |
| --- | --- | --- |
| `github.com/coder/websocket` | v1.8.15 | ISC |
| `github.com/ebitengine/purego` | v0.11.1 | Apache-2.0; Go-derived BSD-3-Clause files |
| `github.com/egoist/mygo` | v0.7.4 | MIT; Apache-2.0 shader code; MingCute and CLDR below |
| `github.com/go-text/typesetting` | v0.3.5 | Unlicense OR BSD-3-Clause; HarfBuzz MIT notices and font/data attributions below |
| `github.com/tidwall/jsonc` | v0.3.3 | MIT |
| `golang.org/x/image` | v0.46.0 | BSD-3-Clause; patent grant |
| `golang.org/x/net` | v0.62.0 | BSD-3-Clause; patent grant; Public Suffix List data below |
| `golang.org/x/text` | v0.43.0 | BSD-3-Clause; patent grant; Unicode data below |
| `google.golang.org/protobuf` | v1.36.12 | BSD-3-Clause; patent grant |

[Module notices](third_party/licenses/modules) retain root and nested license,
notice and patent files, with source-file copyright attributions. The
[Go notice](third_party/licenses/go.txt) retains the standard-library license,
patent grant and vendored notices, including platform/experiment-specific code.

## Data, artwork, references and installers

| Component | Use | Preserved license |
| --- | --- | --- |
| Kivrix | Reference for native layout, local credential storage, update flow and release workflow | [MIT](third_party/licenses/kivrix.txt) |
| Semi Design | Feishu/Lark service mark | [MIT and its retained attributions](third_party/licenses/semi-icons.txt) |
| Ant Design Icons | DingTalk service mark | [MIT](third_party/licenses/ant-design-icons.txt) |
| SVG Logos | Slack and Telegram service marks | [CC0-1.0](third_party/licenses/svg-logos.txt) |
| MingCute | Search icon in MyGo controls | [Apache-2.0](third_party/licenses/mingcute.txt) |
| Unicode CLDR 48.2.0 | MyGo's generated locale tables | [Unicode-3.0](third_party/licenses/unicode-cldr.txt) |
| Unicode character data | Text/language dependencies | [Unicode-3.0](third_party/licenses/unicode-data.txt) |
| FreeType | Routines identified as adaptations in go-text's font renderer | [FTL](third_party/licenses/freetype.txt) |
| Public Suffix List | Domain data embedded by the Windows dependency `x/net/publicsuffix` | [MPL-2.0](third_party/licenses/public-suffix-list.txt) |
| NSIS 3.13 | Windows installer/uninstaller; MyGo uses LZMA compression | [Zlib, bzip2 and CPL-1.0](third_party/licenses/nsis.txt) |
| Feishu/Lark Go SDK | Public wire-protocol reference for the pairing adapter; SDK not bundled | [MIT](third_party/licenses/larksuite-sdk.txt) |
| DingTalk Stream Go SDK | Public protocol reference for pairing; SDK not bundled | [MIT](third_party/licenses/dingtalk-sdk.txt) |

Portions of this software are copyright © 2005-2026 The FreeType Project
(https://freetype.org/). All rights reserved. The FreeType attribution uses the
FTL option; it does not select the alternative GPL license.

The original Public Suffix List and complete unmodified NSIS 3.13 source
archive are included in every package. Their exact revisions, SHA-256 hashes
and source locations are documented in [third_party/sources](third_party/sources).
The MPL applies to the covered data, and the NSIS licenses to their components;
these do not relicense Nudilo's separate application code.

Service marks identify compatible services. Copyright permissions, including
CC0, do not grant trademark rights or imply endorsement. Fixed asset sources
and changes are recorded in the [channel asset notes](internal/desktop/assets/channels/README.md)
and [component registry](third_party/components.json). Nudilo's own navigation
icons are simple line drawings; its AI-generated TOUCH artwork has separate
[provenance notes](design/brand/touch/README.md).

The inspected dependency graph and source references contain no imported
AskHuman implementation. Nudilo does not redistribute Codex or Claude Code
clients, IM clients, system UI libraries or font files from upstream test data.

## Keeping the notices current

`make licenses` checks the reviewed versions, module notices, artwork and
bundled sources against [the inventory](third_party/inventory.json). It also
checks the generated copies without rewriting them. `make check` and packaging
run this check. Dependency upgrades require reviewing
[components.json](third_party/components.json), then explicitly refreshing the
inventory; see [third_party/README.md](third_party/README.md).

Notices are available in macOS app resources, beside the Windows executable,
and in the Linux installation/archive. No in-app license button is required
by this distribution arrangement.
