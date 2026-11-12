#!/usr/bin/env python3
"""Nudilo packaging and release checks, using the standard library and pinned MyGo.

Workflow inspired by Kivrix (MIT); see third_party/licenses/kivrix.txt.
Builds are isolated from the working tree. The main-branch release job stages a
draft and publishes it only after all uploaded assets have been verified.
"""
import argparse
import base64
import copy
import hashlib
import io
import json
import os
from pathlib import Path
import plistlib
import re
import shutil
import struct
import subprocess
import sys
import tarfile
import tempfile
import urllib.error
import urllib.parse
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parent.parent
TARGETS = ('darwin/universal', 'windows/amd64', 'linux/amd64', 'linux/arm64')
ASSETS = ('icon.png', 'logo.png', 'tray.png', 'tray-color.png', 'THIRD_PARTY_NOTICES.txt')
VERSION_PATTERN = r'(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)'


def require(condition, message):
    if not condition:
        raise ValueError(message)


def config(tag=''):
    cfg = json.loads((ROOT / 'mygo.json').read_text(encoding='utf-8'))
    version = cfg['version']
    require(re.fullmatch(VERSION_PATTERN, version), 'Use a stable X.Y.Z version.')
    code = (ROOT / 'internal/buildinfo/version.go').read_text(encoding='utf-8')
    match = re.search(r'const Version = "([^"]+)"', code)
    require(match and match[1] == version, 'mygo.json and internal/buildinfo/version.go must match.')
    require(not tag or tag == 'v' + version, f'Expected v{version}, got {tag}.')
    require(cfg['identifier'] == 'app.callbell.desktop', 'Keep the bundle identifier stable for existing installations.')
    repository = re.search(r'const Repository = "([^"]+)"', code)
    require(repository and cfg.get('updates', {}).get('github') == repository[1], 'mygo.json and buildinfo.Repository must match.')
    require(cfg['updates'].get('deltas') == 0, 'Review the updater and release pipeline before enabling deltas.')
    require(not cfg['updates'].get('privateKey'), 'Keep private-key paths out of mygo.json; use the environment or local release-key directory.')
    public = base64.b64decode(cfg['updates'].get('publicKey', ''), validate=True)
    require(len(public) == 32, 'Configure an Ed25519 update public key.')
    return cfg


def asset_names(version, target):
    label = 'macOS' if target == 'darwin/universal' else target.replace('/', '-')
    base = f'nudilo-{version}-{label}'
    if target.startswith('darwin/'):
        return [base + '.dmg']
    if target.startswith('windows/'):
        return [base + '.exe']
    return [base + '.deb', base + '.tar.gz', 'install.sh']


def update_names(version, target):
    suffix = target.replace('/', '-')
    return [f'nudilo-{version}-{suffix}.tar.gz', f'update-{suffix}.json']


def update_key_environment():
    env = dict(os.environ)
    if not env.get('MYGO_UPDATER_PRIVATE_KEY', '').strip():
        if override := env.get('NUDILO_UPDATE_KEY_FILE'):
            key = Path(override).expanduser()
        else:
            if sys.platform == 'darwin': base = Path.home() / 'Library/Application Support'
            elif os.name == 'nt': base = Path(env.get('AppData', Path.home() / 'AppData/Roaming'))
            else: base = Path(env.get('XDG_CONFIG_HOME', Path.home() / '.config'))
            key = base / 'mygo/update-keys/nudilo/mygo-update.key'
        require(key.is_file(), 'Set MYGO_UPDATER_PRIVATE_KEY or NUDILO_UPDATE_KEY_FILE to the Nudilo release signing key. See docs/UPDATES.md.')
        env['MYGO_UPDATER_PRIVATE_KEY'] = key.read_text(encoding='utf-8').strip()
    # Derive and compare the public key; malformed private-key suffixes are not
    # accepted. No key contents are included in command arguments or output.
    run('go', 'run', './cmd/nudilo-release', '-check-key', cwd=ROOT, env=env)
    return env


def verify_update_signatures(directory, targets=TARGETS):
    run('go', 'run', './cmd/nudilo-release', '-directory', str(directory.resolve()), '-platform', ','.join(targets), cwd=ROOT)


def github_api(path, *, method='GET', data=None, missing_ok=False):
    repo = os.environ['GITHUB_REPOSITORY']
    require(re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repo), 'Invalid repository name.')
    url = f'https://api.github.com/repos/{repo}/{path}'
    request = urllib.request.Request(url, method=method, data=None if data is None else json.dumps(data).encode(), headers={
        'Authorization': 'Bearer ' + os.environ['GH_TOKEN'],
        'Accept': 'application/vnd.github+json', 'User-Agent': 'nudilo-release',
        'Content-Type': 'application/json', 'X-GitHub-Api-Version': '2022-11-28',
    })
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        if missing_ok and method == 'GET' and error.code == 404:
            return None
        raise


def github_list(path):
    items = []
    page = 1
    while True:
        batch = github_api(f'{path}?per_page=100&page={page}')
        require(isinstance(batch, list), f'Expected a list from GitHub: {path}')
        items.extend(batch)
        if len(batch) < 100:
            return items
        page += 1


def tag_commit(tag):
    reference = github_api('git/ref/tags/' + urllib.parse.quote(tag, safe=''), missing_ok=True)
    if reference is None:
        return None
    obj = reference['object']
    # Existing releases may use annotated tags; compare the underlying commit.
    for _ in range(10):
        if obj['type'] == 'commit':
            return obj['sha']
        require(obj['type'] == 'tag', f'{tag} does not point to a commit.')
        obj = github_api('git/tags/' + obj['sha'])['object']
    raise ValueError(f'{tag} has too many nested annotated tags.')


def guard(tag, commit=''):
    version = tuple(map(int, config(tag)['version'].split('.')))
    releases = github_list('releases')
    matches = [item for item in releases if item['tag_name'] == tag]
    require(all(item.get('draft') is True for item in matches), f'{tag} is already published. Use a new version.')
    require(len(matches) <= 1, f'Multiple drafts exist for {tag}; keep only the intended draft.')
    for item in releases:
        match = re.fullmatch('v' + VERSION_PATTERN, item['tag_name'])
        if match and not item['draft'] and not item['prerelease']:
            require(version > tuple(map(int, match.groups())), f'Version must be newer than published {item["tag_name"]}.')
    existing = tag_commit(tag)
    if existing is not None:
        require(commit and existing == commit, f'{tag} already exists for another commit. Use a new version.')
    draft = matches[0] if matches else None
    if draft:
        require(existing and existing == commit, f'{tag} draft must have a tag pointing to the same commit before retrying.')
    return draft, existing


def require_main_push(commit):
    require(os.environ.get('GITHUB_EVENT_NAME') == 'push' and os.environ.get('GITHUB_REF') == 'refs/heads/main',
            'Releases may only be published by a push to main.')
    require(re.fullmatch(r'[0-9a-f]{40}', commit) and commit == os.environ.get('GITHUB_SHA'),
            'Release commit must equal this main workflow commit.')
    require(os.environ.get('GITHUB_REPOSITORY') == config()['updates']['github'],
            'Release repository must match the configured update feed.')


def prepare_tag(tag, commit):
    require_main_push(commit)
    _, existing = guard(tag, commit)
    if existing is None:
        github_api('git/refs', method='POST', data={'ref': 'refs/tags/' + tag, 'sha': commit})
    print(f'Release tag: {tag} at {commit}')


def publish(tag, commit, directory):
    require_main_push(commit)
    verify_assets(directory)
    draft, existing = guard(tag, commit)
    require(draft and existing == commit, f'Expected a draft for {tag} at {commit}.')
    assets = github_list(f'releases/{draft["id"]}/assets')
    files = {p.name: p for p in directory.iterdir() if p.is_file()}
    require(len(assets) == len(files) and {a['name'] for a in assets} == set(files),
            'Uploaded release assets do not match the verified local assets.')
    for asset in assets:
        path = files[asset['name']]
        require(asset['state'] == 'uploaded' and asset['size'] == path.stat().st_size
                and asset.get('digest') == 'sha256:' + digest(path),
                f'Uploaded asset failed size/SHA-256 verification: {path.name}')
    result = github_api(f'releases/{draft["id"]}', method='PATCH', data={'draft': False, 'prerelease': False, 'make_latest': 'true'})
    require(result['tag_name'] == tag and result['draft'] is False and result['prerelease'] is False,
            'GitHub did not confirm a stable published release.')
    print('Published:', result['html_url'])


def run(*args, **kwargs):
    return subprocess.run(args, check=True, **kwargs)


def bundle_locales(source, cfg):
    languages = []
    for file in sorted((source / 'internal/locale/locales').glob('*.json')):
        catalog = json.loads(file.read_text(encoding='utf-8'))
        require(catalog['@@locale'] == file.stem, f'Locale tag mismatch in {file.name}')
        languages.append(file.stem)
        directory = source / 'resources/darwin' / (file.stem + '.lproj')
        directory.mkdir(parents=True, exist_ok=True)
        (directory / 'InfoPlist.strings').write_bytes(plistlib.dumps({'CFBundleDisplayName': cfg['name']}))
    require('en' in languages, 'English source catalog is missing.')
    cfg.setdefault('macos', {}).setdefault('infoPlist', {}).update({
        'CFBundleDevelopmentRegion': 'en', 'CFBundleLocalizations': languages,
    })
    return languages


def signing_options(cfg, targets, signed):
    mac = cfg.setdefault('macos', {})
    mac['signingIdentity'] = os.environ.get('NUDILO_MACOS_SIGNING_IDENTITY', '-')
    if profile := os.environ.get('NUDILO_NOTARY_PROFILE'):
        mac['notarize'] = {'keychainProfile': profile}
    if certificate := os.environ.get('NUDILO_WINDOWS_CERTIFICATE'):
        cfg.setdefault('windows', {})['certificate'] = str(Path(certificate).resolve())
    if signed:
        if 'darwin/universal' in targets:
            require(mac['signingIdentity'].startswith('Developer ID Application:'), '--signed requires a Developer ID Application identity.')
            require(mac.get('notarize', {}).get('keychainProfile'), '--signed requires NUDILO_NOTARY_PROFILE.')
        if 'windows/amd64' in targets:
            require(cfg.get('windows', {}).get('certificate'), '--signed requires NUDILO_WINDOWS_CERTIFICATE.')


def digest(path):
    result = hashlib.sha256()
    with path.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            result.update(chunk)
    return result.hexdigest()


def resource_names(approved):
    registry = json.loads((ROOT / 'third_party/components.json').read_text(encoding='utf-8'))
    sources = registry['bundled_sources']
    require(sources, 'Third-party source inventory is empty.')
    names = list(ASSETS)
    for source in sources:
        path = Path(source)
        require(path.parts[:2] == ('third_party', 'sources') and '..' not in path.parts,
                f'Invalid third-party source path: {source}')
        name = 'third-party-sources/' + path.relative_to('third_party/sources').as_posix()
        require((approved / name).is_file(), f'Missing approved source: {name}')
        names.append(name)
    return names


def verify_resources(directory, approved):
    for name in resource_names(approved):
        require((directory / name).is_file() and digest(directory / name) == digest(approved / name), f'Missing or changed resource: {directory / name}')


def check_elf(data, arch):
    require(len(data) >= 20 and data[:6] == b'\x7fELF\x02\x01', 'Expected a little-endian ELF64 binary.')
    require(struct.unpack_from('<H', data, 18)[0] == {'amd64': 62, 'arm64': 183}[arch], f'Wrong Linux architecture: {arch}')


def check_pe(data):
    require(len(data) > 64 and data[:2] == b'MZ', 'Invalid Windows executable.')
    offset = struct.unpack_from('<I', data, 0x3c)[0]
    require(len(data) >= offset + 6 and data[offset:offset+4] == b'PE\0\0', 'Invalid PE header.')
    require(struct.unpack_from('<H', data, offset+4)[0] == 0x8664, 'Expected a Windows amd64 executable.')


def archive_resources(archive, prefix, approved):
    members = {m.name.removeprefix('./'): m for m in archive.getmembers()}
    for name in resource_names(approved):
        member = members.get(prefix + name)
        require(member is not None and member.isfile(), f'Archive is missing {prefix + name}')
        require(archive.extractfile(member).read() == (approved / name).read_bytes(), f'Changed archive resource: {name}')
    return members


def nsis_version(compiler):
    return subprocess.check_output([str(compiler), '-VERSION'], text=True).strip().removeprefix('v')


def installer_environment(work, targets):
    env = dict(os.environ)
    if 'windows/amd64' not in targets:
        return env
    tool = json.loads((ROOT / 'third_party/components.json').read_text(encoding='utf-8'))['installer_tools']['nsis']
    compiler = shutil.which('makensis')
    if compiler and nsis_version(compiler) == tool['version']:
        return env
    if os.name != 'nt':
        raise ValueError(f"Windows packaging requires reviewed NSIS {tool['version']} (makensis); update its source and license inventory when changing versions.")
    # A runner's preinstalled NSIS can drift independently of Go dependencies.
    # Use the reviewed distribution in the isolated build directory instead.
    archive = work / 'nsis.zip'
    failures = []
    for url in tool['urls']:
        try:
            with urllib.request.urlopen(url, timeout=30) as response, archive.open('wb') as output:
                shutil.copyfileobj(response, output)
            require(digest(archive) == tool['sha256'], 'NSIS download checksum mismatch.')
            break
        except (OSError, ValueError) as error:
            failures.append(str(error))
    else:
        raise ValueError('Cannot download reviewed NSIS: ' + '; '.join(failures))
    dest = work / 'nsis'
    with zipfile.ZipFile(archive) as zipped:
        for member in zipped.infolist():
            path = Path(member.filename)
            require(not path.is_absolute() and '..' not in path.parts and ':' not in member.filename and '\\' not in member.filename,
                    'Invalid NSIS archive path.')
        zipped.extractall(dest)
    candidates = sorted(dest.rglob('makensis.exe'))
    require(candidates, 'Reviewed NSIS archive is missing makensis.exe.')
    compiler = next((p for p in candidates if p.parent.name.lower() == 'bin'), candidates[0])
    require(nsis_version(compiler) == tool['version'], 'Unexpected NSIS compiler version.')
    env['PATH'] = str(compiler.parent) + os.pathsep + env.get('PATH', '')
    return env


def deb_members(path):
    data = path.read_bytes()
    require(data.startswith(b'!<arch>\n'), 'Invalid Debian package.')
    offset, members = 8, {}
    while offset < len(data):
        header = data[offset:offset+60]
        require(len(header) == 60 and header[58:] == b'`\n', 'Invalid Debian member header.')
        size = int(header[48:58])
        require(size >= 0 and offset+60+size <= len(data), 'Truncated Debian package.')
        members[header[:16].decode().strip().rstrip('/')] = data[offset+60:offset+60+size]
        offset += 60+size+size%2
    return members


def verify_macos_icons(app, approved):
    plist = plistlib.loads((app / 'Contents/Info.plist').read_bytes())
    require(plist.get('CFBundleIconFile') == 'Nudilo.icns' and not plist.get('CFBundleIconName'),
            'The macOS app must use its opaque application icon, not the inset volume icon.')
    resources = app / 'Contents/Resources'
    for name, original in (('Nudilo.icns', 'Nudilo.icns'), ('VolumeIcon.icns', 'VolumeIcon.icns'),
                           ('AppIcon.icns', 'VolumeIcon.icns')):
        require((resources / name).is_file() and digest(resources / name) == digest(approved / 'darwin' / original),
                f'Missing or changed macOS icon: {name}')


def verify_dmg_icons(dmg, approved):
    # Verify the actual installer, including MyGo's custom-volume-icon flag.
    # Mount privately so packaging never opens a Finder window on the host.
    with tempfile.TemporaryDirectory(prefix='nudilo-dmg-check-') as temp:
        mount = Path(temp) / 'volume'
        mount.mkdir()
        run('hdiutil', 'attach', str(dmg), '-readonly', '-nobrowse', '-noautoopen',
            '-mountpoint', str(mount), stdout=subprocess.DEVNULL)
        try:
            icon = mount / '.VolumeIcon.icns'
            require(icon.is_file() and digest(icon) == digest(approved / 'darwin/VolumeIcon.icns'),
                    'DMG must use the rounded, padded volume icon.')
            finder_info = bytes.fromhex(subprocess.check_output(
                ['xattr', '-px', 'com.apple.FinderInfo', str(mount)], text=True))
            require(len(finder_info) >= 10 and struct.unpack_from('>H', finder_info, 8)[0] & 0x0400,
                    'DMG custom volume icon is not enabled.')
            verify_macos_icons(mount / 'Nudilo.app', approved)
        finally:
            try:
                run('hdiutil', 'detach', str(mount), stdout=subprocess.DEVNULL)
            except subprocess.CalledProcessError:
                run('hdiutil', 'detach', '-force', str(mount), stdout=subprocess.DEVNULL)


def verify_package(cfg, target, directory, approved, languages, signed=False):
    version, arch = cfg['version'], target.split('/')[1]
    if target.startswith('darwin/'):
        app = directory / 'Nudilo.app'
        plist = plistlib.loads((app / 'Contents/Info.plist').read_bytes())
        require(plist['CFBundleShortVersionString'] == version, 'Wrong macOS version.')
        require(plist['CFBundleLocalizations'] == languages, 'Missing macOS languages.')
        verify_resources(app / 'Contents/Resources', approved)
        verify_macos_icons(app, approved)
        for file in (app / 'Contents/MacOS/Nudilo', app / 'Contents/Resources/bin/callbell-hook'):
            arches = subprocess.check_output(['lipo', '-archs', str(file)], text=True).split()
            require(set(arches) == {'arm64', 'x86_64'}, f'Not a universal binary: {file}')
        run('codesign', '--verify', '--deep', '--strict', str(app))
        dmg = directory / f'nudilo-{version}.dmg'
        run('hdiutil', 'verify', str(dmg), stdout=subprocess.DEVNULL)
        verify_dmg_icons(dmg, approved)
        if signed:
            run('xcrun', 'stapler', 'validate', str(dmg))
            run('spctl', '--assess', '--type', 'execute', str(app))
        return [dmg]
    if target.startswith('windows/'):
        check_pe((directory / 'Nudilo.exe').read_bytes())
        check_pe((directory / 'bin/callbell-hook.exe').read_bytes())
        verify_resources(directory, approved)
        installer = directory / f'nudilo-setup-{version}-amd64.exe'
        require(installer.is_file() and installer.stat().st_size > 65536, 'Missing Windows installer.')
        if os.name == 'nt':
            command = "if ((Get-Item -LiteralPath $env:VERIFY_APP).VersionInfo.ProductVersion -ne $env:VERIFY_VERSION) { throw 'Wrong version' }; "
            if signed:
                command += "foreach ($p in @($env:VERIFY_APP,$env:VERIFY_SETUP)) { if ((Get-AuthenticodeSignature -LiteralPath $p).Status -ne 'Valid') { throw 'Invalid signature' } }"
            run('powershell', '-NoProfile', '-NonInteractive', '-Command', command,
                env=dict(os.environ, VERIFY_APP=str(directory / 'Nudilo.exe'), VERIFY_SETUP=str(installer), VERIFY_VERSION=version))
        elif signed:
            raise ValueError('Verify signed Windows packages on Windows.')
        return [installer]
    archive = directory / f'nudilo-{version}-linux-{arch}.tar.gz'
    with tarfile.open(archive) as tar:
        members = archive_resources(tar, '', approved)
        for file in ('nudilo', 'bin/callbell-hook'):
            check_elf(tar.extractfile(members[file]).read(20), arch)
    deb = directory / f'nudilo_{version}_{arch}.deb'
    members = deb_members(deb)
    with tarfile.open(fileobj=io.BytesIO(members['control.tar.gz'])) as tar:
        control = next(m for m in tar.getmembers() if m.name.removeprefix('./') == 'control')
        text = tar.extractfile(control).read().decode()
        require(f'Version: {version}\n' in text and f'Architecture: {arch}\n' in text, 'Wrong Debian metadata.')
        require('libwebkit' not in text, 'Native packages must not depend on WebKitGTK.')
    with tarfile.open(fileobj=io.BytesIO(members['data.tar.gz'])) as tar:
        archive_resources(tar, 'opt/nudilo/', approved)
    installer = directory / 'install.sh'
    require(installer.is_file(), 'Missing Linux installer script.')
    run('sh', '-n', str(installer))
    return [deb, archive, installer]


def package(args):
    cfg = copy.deepcopy(config())
    targets = list(dict.fromkeys(args.platform.split(',')))
    require(targets and all(t in TARGETS for t in targets), 'Supported platforms: ' + ', '.join(TARGETS))
    output = (ROOT / args.out).resolve()
    require(output != ROOT and not ROOT.is_relative_to(output), 'Use a separate output directory, not a parent of the project.')
    signing_options(cfg, targets, args.signed)
    key_env = update_key_environment()
    run('go', 'run', './cmd/callbell-assets', '-check-licenses', cwd=ROOT)
    with tempfile.TemporaryDirectory(prefix='nudilo-package-') as work:
        build_env = installer_environment(Path(work), targets)
        build_env['MYGO_UPDATER_PRIVATE_KEY'] = key_env['MYGO_UPDATER_PRIVATE_KEY']
        source = Path(work) / 'source'
        source.mkdir()
        # Only package source inputs. Personal data, keys and previous builds
        # are never copied into the temporary source snapshot.
        for name in ('main.go', 'go.mod', 'go.sum', 'cmd', 'internal', 'resources', 'LICENSE', 'third_party', 'design/brand/touch'):
            original, destination = ROOT / name, source / name
            destination.parent.mkdir(parents=True, exist_ok=True)
            if original.is_dir():
                shutil.copytree(original, destination, ignore=shutil.ignore_patterns('darwin-*', 'windows-*', 'linux-*', '__pycache__'))
            else:
                shutil.copy2(original, destination)
        run('go', 'run', './cmd/callbell-assets', cwd=source)
        languages = bundle_locales(source, cfg)
        (source / 'mygo.json').write_text(json.dumps(cfg, indent=2) + '\n', encoding='utf-8')
        for target in targets:
            goos, arch = target.split('/')
            for cpu in ('amd64', 'arm64') if arch == 'universal' else (arch,):
                helper = source / 'resources' / f'{goos}-{cpu}/bin' / ('callbell-hook.exe' if goos == 'windows' else 'callbell-hook')
                helper.parent.mkdir(parents=True, exist_ok=True)
                run('go', 'build', '-trimpath', '-ldflags=-s -w', '-o', str(helper), './cmd/callbell-hook',
                    cwd=source, env=dict(os.environ, GOOS=goos, GOARCH=cpu, CGO_ENABLED='0'))
        command = ['go', 'tool', 'mygo', 'build', '-platform', ','.join(targets), '-o', str(output)]
        if not args.signed:
            command.append('-skip-notarize')
        run(*command, cwd=source, env=build_env)
        release = output / 'release'
        release.mkdir(parents=True, exist_ok=True)
        prepared = {}
        for target in targets:
            directory = output / target.replace('/', '-')
            paths = verify_package(cfg, target, directory, source / 'resources', languages, args.signed)
            for name, path in zip(asset_names(cfg['version'], target), paths):
                if name in prepared:
                    require(digest(path) == digest(prepared[name]), f'Conflicting release asset: {name}')
                prepared[name] = path
            verify_update_signatures(directory, [target])
            archive, manifest = update_names(cfg['version'], target)
            with tarfile.open(directory / archive) as tar:
                prefix = 'Nudilo.app/Contents/Resources/' if target.startswith('darwin/') else ''
                members = archive_resources(tar, prefix, source / 'resources')
                binaries = ['Nudilo.app/Contents/MacOS/Nudilo', 'Nudilo.app/Contents/Resources/bin/callbell-hook'] if target.startswith('darwin/') else ['Nudilo.exe', 'bin/callbell-hook.exe'] if target.startswith('windows/') else ['nudilo', 'bin/callbell-hook']
                for name in binaries:
                    member = members.get(name)
                    require(member is not None and member.isfile(), f'Update archive is missing {name}')
                    require(hashlib.sha256(tar.extractfile(member).read()).hexdigest() == digest(directory / name), f'Update binary differs from installer: {name}')
            for name in (archive, manifest): prepared[name] = directory / name
        for name, path in prepared.items():
            shutil.copy2(path, release / name)
        write_checksums(release)
        print('Verified release assets:', release)


def write_checksums(directory):
    files = sorted(f for f in directory.iterdir() if f.is_file() and f.name != 'SHA256SUMS')
    require(files and all(f.stat().st_size for f in files), 'Release assets are missing or empty.')
    (directory / 'SHA256SUMS').write_text(''.join(f'{digest(f)}  {f.name}\n' for f in files), encoding='utf-8')


def verify_assets(directory):
    version = config()['version']
    expected = {name for target in TARGETS for name in asset_names(version, target) + update_names(version, target)}
    files = [p for p in directory.iterdir() if p.is_file() and p.name != 'SHA256SUMS']
    require({p.name for p in files} == expected, 'Release must contain exactly: ' + ', '.join(sorted(expected)))
    verify_update_signatures(directory)
    write_checksums(directory)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='command', required=True)
    check = sub.add_parser('check')
    check.add_argument('--tag', default=os.environ.get('GITHUB_REF_NAME', '') if os.environ.get('GITHUB_REF_TYPE') == 'tag' else '')
    check.add_argument('--github-output', type=Path)
    for command in ('guard', 'prepare-tag', 'publish'):
        gate = sub.add_parser(command)
        gate.add_argument('--tag', required=True)
        gate.add_argument('--commit', required=command != 'guard', default='')
        if command == 'publish': gate.add_argument('directory', type=Path)
    build = sub.add_parser('package')
    build.add_argument('--platform', required=True)
    build.add_argument('--out', '-o', default='dist')
    build.add_argument('--signed', action='store_true')
    verify = sub.add_parser('verify'); verify.add_argument('directory', type=Path)
    args = parser.parse_args()
    if args.command == 'check':
        version = config(args.tag)['version']
        if args.github_output:
            with args.github_output.open('a', encoding='utf-8') as output:
                output.write(f'version={version}\ntag=v{version}\n')
        print('Release version:', version)
    elif args.command == 'guard': guard(args.tag, args.commit)
    elif args.command == 'prepare-tag': prepare_tag(args.tag, args.commit)
    elif args.command == 'publish': publish(args.tag, args.commit, args.directory)
    elif args.command == 'package': package(args)
    else: verify_assets(args.directory)


if __name__ == '__main__':
    try: main()
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as error:
        raise SystemExit(str(error))
