# 开源许可检查

[English](LICENSING.md)

检查日期：**2026-11-09**。范围为 Nudilo 0.1.0，以及[许可清单](../third_party/inventory.json)中锁定的依赖版本。

## 结论

Nudilo 自有代码继续使用现有的 [MIT 许可证](../LICENSE)。版权声明、授权条款和免责条款完整，不需要为了本次开源准备更换主许可证。在已检查的应用依赖中，没有发现要求把整个 Nudilo 改为 GPL 的情况。

不过，不能把整个安装包笼统地说成“全部采用 MIT”。第三方代码、数据、图标及安装器分别保留各自的许可；这次修改主要补齐这些部分，并防止发版时遗漏。

MIT 允许商用、修改和再分发，也允许别人将代码用于闭源产品，但需要保留版权及许可声明。**MIT 不要求别人公开修改后的代码。**这是项目当前的许可选择，本次没有改变。原文见 [OSI MIT](https://opensource.org/license/mit)。

## 检查范围与已处理问题

检查了 Go 模块图，以及应用、通知助手、项目测试和固定版本 MyGo 构建工具在五个目标上的实际依赖：macOS Intel / Apple Silicon、Windows x64、Linux x64 / ARM64。另检查了嵌入资源、图标来源、参考代码署名、Go 标准库许可和打包脚本。模块图中被选中但没有参与上述编译的模块单独记录，不把它们误报为随应用分发。

| 发现的问题 | 本次处理 |
| --- | --- |
| 旧收集器主要读取依赖根目录，可能漏掉子目录许可。 | 收集嵌套许可、NOTICE、专利授权及源码版权署名，保留 HarfBuzz 和 MyGo shader 等声明。 |
| 语言和字符表有独立的数据许可。 | 补上 Unicode / CLDR 许可，保留 CLDR 48.2.0 原有版权年份。 |
| 字体处理代码标注了对 FreeType 例程的改编。 | 补齐 FreeType License 和致谢，同时保留 go-text 自身声明。 |
| Windows 依赖包含 MPL-2.0 的 Public Suffix List 数据。 | 附带完整 MPL 文本、确切版本的原始列表和源码获取说明。 |
| Windows 安装器实际分发 NSIS 代码，并使用 LZMA。 | 附带 NSIS 各项许可和完整的 3.13 源码压缩包，打包时核对编译器版本。 |
| 图标及参考代码的分支链接会随上游更新。 | 记录固定提交、图标原文件哈希及改动说明，保留 Kivrix 与协议参考 SDK 的许可。 |
| 升级依赖后容易忘记更新许可。 | CI 和打包前检查版本、许可文件、素材哈希及源码副本，打包后再核对资源。 |
| Windows Git 检出可能自动改变换行符。 | 使用 `.gitattributes` 保留第三方许可及源码的原始字节。 |

[第三方许可概览](../THIRD_PARTY_NOTICES.md)列出各组件的用途与许可；[完整许可文本](../resources/THIRD_PARTY_NOTICES.txt)随安装包分发，不只是放几个上游链接。

## 几类需要单独说明的许可

**MIT、BSD、ISC、Apache 与 Unicode：**保留完整版权、许可与适用的专利授权和 NOTICE。修改过的上游图标标明改动。项目根目录的 MIT 不会替代第三方要求。参考 [Apache 2.0](https://www.apache.org/licenses/LICENSE-2.0) 和 [Unicode 许可](https://www.unicode.org/license.txt)。

**Public Suffix List：**Windows 间接依赖嵌入了使用 MPL-2.0 的域名后缀数据。安装包中的 `third-party-sources/public_suffix_list.dat` 提供确切版本的原始列表，许可说明还列出生成器与 Go 表示的来源。Nudilo 没有修改列表。MPL 对这部分数据的源码要求不等于把独立的 Nudilo 应用整体改成 MPL，见 [Mozilla 官方 FAQ](https://www.mozilla.org/en-US/MPL/2.0/FAQ/)。

**NSIS：**不能把它仅当作“构建时用一下的工具”；Windows 安装程序和卸载程序包含它的代码。MyGo 选择的 LZMA 组件使用 CPL-1.0。安装包附带完整、未修改的 `third-party-sources/nsis-3.13-source.tar.gz`，其中包含解码器和构建脚本，同时保留原有许可并说明来源。这些条件针对安装器组件，不改变 Nudilo 自有代码的 MIT。见 [NSIS 官方许可说明](https://nsis.sourceforge.io/License)。

本次没有修改这些上游许可证。如果以后改动受这些许可约束的代码或数据，需要标明改动，并提供相应许可要求的源码及声明。

## Logo、平台名称与贡献

飞书、Lark、钉钉、Slack、Telegram 图标保留各自的上游版权许可。商标权与版权许可是两回事：图标用来说明兼容的平台，不表示官方授权、合作或背书。特别是 [CC0](https://creativecommons.org/publicdomain/zero/1.0/legalcode) 也不授予商标权。发布宣传材料时还应遵循相应品牌规则，例如 [Slack 品牌资料](https://slack.com/media-kit) 和 [Telegram 标志说明](https://telegram.org/press)。

Nudilo 的 TOUCH 品牌素材来自图像生成与编辑，提示词和确认版本保留在 [design/brand](../design/brand/README.md)。项目自有代码、文档及素材在贡献者拥有相关权利的范围内适用根目录 MIT，另有许可的第三方材料除外。这不保证 AI 生成图形具有独占版权，也不授予商标权。本次没有进行 Nudilo 名称、slogan 或 Logo 的商标检索。

[贡献说明](../CONTRIBUTING.md)补充了：有意提交给项目的贡献默认按 MIT 提供，另行标注的第三方材料除外；贡献者需要有权提交相关内容，并保留第三方声明。本次没有额外引入 CLA 或强制 DCO 签署流程。

## 后续维护

日常运行 `make licenses` 检查许可清单。升级依赖、替换图标、更新 Go 或 Windows 安装器时，先查看上游许可及改动，更新[组件登记文件](../third_party/components.json)，然后运行：

```sh
go run ./cmd/callbell-assets -refresh-licenses
make check
```

检查生成清单和许可文件的差异。更新哈希本身不能证明新依赖已经获得使用许可。打包流程会带上许可及相关源码副本并核对内容，详见[发版说明](RELEASING.md)和[源码来源说明](../third_party/sources/README.md)。

本次是针对所列版本的源码、依赖及分发检查，不涵盖专利检索、商标清查、每位贡献者的权属证明或第三方平台账号/API 的服务条款；后续升级仍需检查。本次工作不会公开 GitHub 仓库、发布版本，也不会改变 Codex、Claude Code 或 IM 客户端自身的许可。
