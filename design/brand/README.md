# Nudilo brand assets

**01 / TOUCH** was approved on 2026-11-04. The application icon, sidebar,
tray and project README use this version. Superseded concepts and intermediate
exports have been removed; the [approved files and generation prompts](touch/README.md)
remain the source of truth.

| Asset | Use |
| --- | --- |
| [Application icon](touch/nudilo-app-icon.png) | Fully opaque warm-white background; the mark occupies approximately 72.8% of the width. |
| [Transparent logo](touch/nudilo-logo.png) | Sidebar and README; no wordmark or slogan inside the image. |
| [Tray template](touch/nudilo-tray-template.png) | Black and transparent artwork for macOS and Windows. |
| [Color tray icon](touch/nudilo-tray-color.png) | Coral artwork for Linux AppIndicator. |
| [Installer volume icon](touch/nudilo-volume-icon.png) | macOS mounted DMG; inset warm-white rounded square with transparent surroundings. |

`cmd/callbell-assets` copies these four files to `resources/icon.png`,
`resources/logo.png`, `resources/tray.png` and `resources/tray-color.png`.
The copies are packaging inputs. Keep the approved originals when cleaning
build output.

For macOS, the same command also encodes `resources/darwin/Nudilo.icns`
from the opaque application original and `resources/darwin/VolumeIcon.icns`
from the separate installer artwork, with PNG representations from 32 to
1024 pixels. The volume tile occupies approximately 78% of the canvas width;
the transparent padding and rounded corners are part of that image.

MyGo v0.7.4 always copies the bundle's `AppIcon.icns` to the DMG's
`.VolumeIcon.icns`. `macos.icon` therefore supplies the volume ICNS, and
`macos.infoPlist.CFBundleIconFile` explicitly selects `Nudilo.icns` for the
application. This keeps the application's original full-bleed framing while
giving the ejectable desktop volume its own smaller silhouette. Release
verification checks both the app and the mounted DMG to catch changes to this
MyGo behavior. All icon resources are prepared before signing.

The target brand color is coral **#E48A73**, with a light warm-white application
icon background. These are generated raster assets, so individual pixels vary
slightly. The slogan is displayed separately: **A little nudge from your AI.**

## macOS menu-bar sizing

The tray PNG is 1419 × 1108 pixels. MyGo v0.7.4 displays it in an approximately
23.05 × 18 pt image frame. The visible mark is approximately **18.08 × 13.19 pt**,
measured at alpha > 128; the surrounding transparent padding is intentional.
The image frame is separate from the status item's clickable area.

The 18 pt frame is MyGo's implementation choice, and the visible mark's size
was chosen through visual review. Apple does not prescribe that size for every
custom menu-bar icon. Sizing and template-image guidance reviewed on 2026-11-04:
[Menu bar extras](https://developer.apple.com/design/human-interface-guidelines/the-menu-bar#Menu-bar-extras),
[Icons](https://developer.apple.com/design/human-interface-guidelines/icons),
[NSImage.isTemplate](https://developer.apple.com/documentation/appkit/nsimage/istemplate).

## License and provenance

The [TOUCH record](touch/README.md) documents image generation and editing.
Nudilo offers its own artwork under the root [MIT terms](../../LICENSE) to the
extent its contributors hold applicable rights. This does not promise exclusive
copyright in AI-generated shapes or grant trademark rights. The name, slogan
and logo have not undergone trademark clearance in this review. Third-party
service marks retain their separate [notices](../../THIRD_PARTY_NOTICES.md).
