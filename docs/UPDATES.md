# Automatic updates

[简体中文](UPDATES.zh-CN.md)

Nudilo uses MyGo's signed native updater, with a dedicated Nudilo Ed25519 key
and the `cuikaipeng/nudilo` GitHub release feed. Kivrix's update flow informed
the integration; its keys and feed are not reused.

## Using updates

- **Settings → Updates** shows the running version, automatic-update switch,
  check result, download progress and restart action.
- On macOS, choose **Nudilo → Check for updates**, below **About Nudilo** in the
  application menu. On Windows and Linux, use **File → Check for updates**.
  The tray menu also offers **Check for updates**.
- Automatic updates are enabled by default, including when older preferences
  are migrated. A background check runs 30 seconds after launch, then every
  six hours while Nudilo is running. Turning the switch on also checks now.
- A newer stable release is downloaded and verified automatically. Nudilo
  continues processing notifications; it never restarts automatically.
- **Restart to update** activates the prepared version. Closing the window
  keeps Nudilo in the tray; it is not a restart. A normal quit/relaunch can also
  activate an update. Unsaved channel/notification/project-rule drafts require confirmation
  before an update-triggered restart.
- With automatic updates off, use **Check for updates → Download update →
  Restart to update**. Turning the switch off prevents new background checks
  and downloads; a preparation already in progress is allowed to complete.
- Failed checks and failed downloads have separate states. A failed download
  or signature verification can be retried. Concurrent clicks do not start
  duplicate checks or installs. The interface follows the current language
  and light/dark theme.

Updates use HTTPS and the system proxy, with redirects to HTTP rejected.
Requests carry no IM credentials, notification history, project names or
GitHub account tokens. Only stable public releases are considered, not drafts
or prereleases. A missing/private/unpublished feed is shown as **No public
update is available yet**, not “up to date.”

The app and notification helper are updated together. Local settings, encrypted
credentials, history and assistant hooks remain outside the installation;
startup reconnects enabled assistants using the new helper at the same stable
local path.

## Platforms and installation locations

| Installation | Behavior |
| --- | --- |
| macOS universal app in a writable installed location | Signed update is prepared; MyGo replaces the bundle at quit/relaunch. |
| Windows x64 in a user-writable installation directory | Signed files are prepared in place; restarting runs the new executable. |
| Linux x64/ARM64 installed from a tarball into a user-owned directory | Signed files update in place; restarting runs the new version. |
| Read-only locations, package-manager installs such as system-owned `.deb`, or development builds | Still checks for newer versions; offers the official download page instead of self-installing. |

Nudilo never requests elevated privileges to replace a system-owned install.
Use the same package manager or install a newer package there. All release
archives include the helper and the complete third-party notice/source bundle.

## Release signing setup

The public key is in `mygo.json → updates.publicKey`; the app's embedded trusted
key comes from that configuration. Each archive is signed over its SHA-256
using Ed25519. An OS certificate / Apple notarization is separate from this
update signature. Update signing is required even for ad-hoc macOS or unsigned
Windows installer builds.

For this checkout, a dedicated private/public pair was generated outside the
repository at:

- macOS: `~/Library/Application Support/mygo/update-keys/nudilo/`
- The private file is `mygo-update.key`; the public file is `mygo-update.pub`.

Back up the private key privately. Keep the same key for future releases:
installed versions trust its matching public key. Do not commit it, paste it
into issues, or replace it with a newly generated key during an ordinary build.
The packaging script validates the private seed and derived public key before
building and never prints their values.

Local packaging reads the key in this order:

1. `MYGO_UPDATER_PRIVATE_KEY` environment variable (base64 key contents).
2. `NUDILO_UPDATE_KEY_FILE` environment variable (path to the private file).
3. The OS configuration directory's `mygo/update-keys/nudilo/mygo-update.key`
   (`~/Library/Application Support` on macOS, `%AppData%` on Windows,
   `$XDG_CONFIG_HOME` or `~/.config` on Linux).

For GitHub Actions, set the repository Actions secret
**`MYGO_UPDATER_PRIVATE_KEY`** to this key's contents. The packaging job already
reads that secret. PR checks require no key; release packaging fails clearly
when the matching key is missing. The private key is never included in release
assets. Once configured, pushes to `main` can publish signed updates automatically.

Forks must use their own key and repository. Generate a key with
`go tool mygo keygen -o /a/private/directory`, update `updates.publicKey` and
`updates.github`, and keep `internal/buildinfo/version.go → Repository` in
sync. Existing Nudilo installations will not trust that fork's key.

## Publish an update

1. Configure the repository secret once and back up the key.
2. Work on `dev`. Update the version in `mygo.json` and
   `internal/buildinfo/version.go` together to a higher stable version.
3. Run `make check`. Use a manual Actions run on `dev` to package all platforms
   without publishing, and test the final packages on their target systems.
   Local packaging uses the same pipeline and does not replace a running app.
4. Review the bilingual release notes, then merge the `dev` → `main` release PR.
   CI checks the version, builds every platform, verifies signatures, creates
   `vX.Y.Z` at that exact `main` commit, and uploads all assets to a draft. Once
   the uploaded files pass size and SHA-256 checks, CI automatically publishes
   the stable release as latest. There is no manual tagging or publication step.
   The repository/release must be publicly downloadable for the anonymous app feed.
5. Every platform's installer, signed archive and manifest is published together.
   The manifest names are fixed: `update-darwin-universal.json`,
   `update-windows-amd64.json`, `update-linux-amd64.json` and
   `update-linux-arm64.json`. Archives use
   `nudilo-X.Y.Z-<platform>-<architecture>.tar.gz`.

The pipeline checks signatures cryptographically, verifies exact feed URLs,
versions and sizes, compares app/helper bytes with installer staging, and
verifies notice/source resources inside update archives. Missing or tampered
updates fail the build. `SHA256SUMS` covers installers and update assets.
Deltas are disabled for now, so each update is a complete signed archive.
Ordinary `dev` pushes and PRs only run checks. Published versions cannot be
overwritten or replaced by an older version; incomplete uploads stay as drafts.
See [release workflow and retries](RELEASING.md#version-and-github-workflow).

The initial 0.1.0 package establishes the key/feed. An installation will only
offer a release with a **higher version**. Before a public release is present,
network requests cannot exercise the live end-to-end update path; local tests
cover state transitions, secure requests, signature verification, artifact
contents and both UI languages without updating a real installation.

Upstream details: [MyGo v0.7.4 update documentation](https://github.com/egoist/mygo/blob/v0.7.4/docs/updates.md).
