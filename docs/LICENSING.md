# Open-source licensing review

[简体中文](LICENSING.zh-CN.md)

Reviewed on **2026-11-09**, for Nudilo 0.1.0 and the dependency versions recorded
in [the inventory](../third_party/inventory.json).

## Conclusion

Keep the existing [MIT license](../LICENSE) for Nudilo's own code. Its copyright
notice, permission grant and warranty disclaimer are complete. The inspected
application dependency graph does not require relicensing the whole app under
GPL. Some bundled components have their own conditions, so describing the
entire distribution as “all MIT” would be inaccurate.

MIT permits commercial use, modification and redistribution, including in
closed-source products, provided its copyright and permission notice are
retained. It does **not** require downstream modifications to be published.
This is the existing project's licensing choice; this review does not change it.
See the [OSI MIT text](https://opensource.org/license/mit).

## Scope and findings

The review inspected the Go module graph and app/helper/test/build-tool imports
for five targets: `darwin/amd64`, `darwin/arm64`, `windows/amd64`, `linux/amd64`
and `linux/arm64`. It also inspected embedded resources, icon origins,
reference-code notices, Go standard-library notices and the packaging scripts.
Selected but unused modules are recorded separately from compiled modules.

| Finding | Resolution |
| --- | --- |
| The old collector mostly read module-root licenses. | Collect nested license/notice/patent files and source copyright attributions; preserve HarfBuzz and MyGo shader notices. |
| Generated locale and character data had separate terms. | Include Unicode/CLDR notices, retaining the original CLDR 48.2.0 copyright text. |
| A font routine identifies a FreeType adaptation. | Retain the FreeType License and required credit alongside go-text's own notices. |
| Windows embeds Public Suffix List data under MPL-2.0. | Include the MPL text, original list at its exact revision and a source-availability statement. |
| The Windows installer contains NSIS code and uses LZMA. | Include NSIS notices and its complete 3.13 source archive; require a matching compiler when packaging. |
| Code/artwork provenance could move with upstream branches. | Record immutable reference revisions, SVG source hashes and local adaptations; retain Kivrix and protocol-reference licenses. |
| Future upgrades could silently invalidate notices. | Check the reviewed dependency versions, notice/source bytes and asset hashes in CI and before packaging; verify bundled copies. |
| Windows Git checkout can alter line endings. | Preserve exact third-party license/source bytes with `.gitattributes`. |

The [third-party overview](../THIRD_PARTY_NOTICES.md) lists licenses and uses.
The [full notice bundle](../resources/THIRD_PARTY_NOTICES.txt) is the
redistributable text, not merely a collection of links.

## Component-specific obligations

**MIT, BSD, ISC, Apache and Unicode components.** Retain their full license and
copyright notices, applicable patent grants and Apache notices. Changes to
upstream artwork are identified. The project's root MIT license does not erase
these obligations. See [Apache 2.0](https://www.apache.org/licenses/LICENSE-2.0)
and [Unicode's license](https://www.unicode.org/license.txt).

**Public Suffix List.** The embedded domain data has MPL-2.0 terms. The original
list is shipped in `third-party-sources/public_suffix_list.dat`, and the
notice identifies the upstream generator/Go representation. Nudilo does not
modify the list. This component's source obligations do not turn the separate
Nudilo application into MPL software; see the [Mozilla MPL FAQ](https://www.mozilla.org/en-US/MPL/2.0/FAQ/).

**NSIS.** The installer is not simply an external build tool: its engine and
uninstaller are distributed in Windows packages. The selected LZMA component
has CPL-1.0 terms. The unmodified NSIS 3.13 source, including the decoder and
build scripts, is supplied in `third-party-sources/nsis-3.13-source.tar.gz`.
The notices retain its separate terms and identify the source. These
conditions concern the installer components, not a change to Nudilo's MIT
license. See the [NSIS licensing page](https://nsis.sourceforge.io/License).

No change to these third-party licenses has been made. Future modifications
to covered data/code must be identified and accompanied by the corresponding
source and notices required by the component's license.

## Artwork, names and contributions

Service icons retain their upstream copyright permissions. Their owners'
trademark rights are separate; use them to identify integrations without
claiming sponsorship. In particular, [CC0](https://creativecommons.org/publicdomain/zero/1.0/legalcode)
does not grant trademark rights. Consult service-specific brand guidance,
such as [Slack's media kit](https://slack.com/media-kit) and
[Telegram's logo guidance](https://telegram.org/press).

Nudilo's TOUCH assets were generated and edited using image generation; the
prompts and approved files are preserved in [design/brand](../design/brand/README.md).
Nudilo-authored code, docs and artwork are offered under the root MIT terms
to the extent the contributors hold applicable rights, excluding separately
licensed third-party material. This does not promise exclusive copyright in
AI-generated shapes or grant trademark rights. This review did not perform
a trademark clearance search for the name, slogan or logo.

[Contributing](../CONTRIBUTING.md) now states that intentional contributions
are under MIT unless explicitly identified otherwise, and that contributors
must retain third-party notices and have the right to contribute their work.
No separate CLA or mandatory DCO process is introduced.

## Maintenance and limits

Run `make licenses` to verify the current inventory. For dependency, icon,
Go-toolchain or installer upgrades, inspect the new upstream terms and changes
first, update [the registry](../third_party/components.json), then run:

```sh
go run ./cmd/callbell-assets -refresh-licenses
make check
```

Review the resulting inventory and notice diff. Refreshing a checksum does
not establish permission to use a new dependency. Packaging includes and
checks the notices and covered source copies; see [release instructions](RELEASING.md)
and [source provenance](../third_party/sources/README.md).

This is a source/dependency/distribution review of the listed versions, not a
guarantee about patents, trademark clearance, every contributor's ownership,
or the terms of third-party accounts and APIs. Future dependency or asset
changes need another review. This work does not publish the repository or a
release, and it does not change the license of third-party clients.
