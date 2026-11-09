import importlib.util
import io
import json
from pathlib import Path
import struct
import tarfile
import tempfile
import unittest
from unittest.mock import patch
import urllib.error

spec = importlib.util.spec_from_file_location('release', Path(__file__).with_name('release.py'))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseTests(unittest.TestCase):
    def test_packaged_notices_and_source_copies_are_required(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            approved, package = root / 'approved', root / 'package'
            sources = json.loads((release.ROOT / 'third_party/components.json').read_text())['bundled_sources']
            names = list(release.ASSETS) + ['third-party-sources/' + Path(p).name for p in sources]
            for folder in (approved, package):
                for name in names:
                    path = folder / name; path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_bytes(name.encode())
            release.verify_resources(package, approved)
            source = package / names[-1]
            source.write_bytes(b'tampered source archive')
            with self.assertRaisesRegex(ValueError, 'Missing or changed resource'):
                release.verify_resources(package, approved)
            source.unlink()
            with self.assertRaises(ValueError): release.verify_resources(package, approved)
            with tarfile.open(root / 'fixture.tar', 'w') as archive:
                for name in names[:-1]: archive.add(package / name, arcname='opt/nudilo/' + name)
            with tarfile.open(root / 'fixture.tar') as archive:
                with self.assertRaisesRegex(ValueError, 'Archive is missing'):
                    release.archive_resources(archive, 'opt/nudilo/', approved)

    def test_windows_packaging_rejects_unreviewed_nsis_on_cross_build_host(self):
        tool = json.loads((release.ROOT / 'third_party/components.json').read_text())['installer_tools']['nsis']
        with patch.object(release.shutil, 'which', return_value='makensis'), patch.object(release, 'nsis_version', return_value=tool['version']):
            release.installer_environment(Path('.'), ['windows/amd64'])
        with patch.object(release.os, 'name', 'posix'), patch.object(release.shutil, 'which', return_value='makensis'), patch.object(release, 'nsis_version', return_value='3.0'):
            with self.assertRaisesRegex(ValueError, 'reviewed NSIS'):
                release.installer_environment(Path('.'), ['windows/amd64'])
            # Non-Windows packages have no dependency on an installed compiler.
            release.installer_environment(Path('.'), ['linux/amd64'])

    def test_version_and_tag_must_match(self):
        version = release.config()['version']
        self.assertEqual(release.config('v' + version)['version'], version)
        with self.assertRaises(ValueError): release.config('v999.0.0')

    def test_languages_come_from_catalogs(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            catalogs = root / 'internal/locale/locales'; catalogs.mkdir(parents=True)
            for tag in ('en', 'zh-Hans', 'ja'):
                (catalogs / (tag + '.json')).write_text(json.dumps({'@@locale': tag}))
            cfg = {'name': 'Nudilo'}
            self.assertEqual(release.bundle_locales(root, cfg), ['en', 'ja', 'zh-Hans'])
            self.assertEqual(cfg['macos']['infoPlist']['CFBundleDevelopmentRegion'], 'en')
            self.assertTrue((root / 'resources/darwin/ja.lproj/InfoPlist.strings').exists())

    def test_release_requires_every_platform_and_nonempty_assets(self):
        with tempfile.TemporaryDirectory() as temp, patch.object(release, 'verify_update_signatures') as signatures:
            root = Path(temp); version = release.config()['version']
            for target in release.TARGETS:
                for name in release.asset_names(version, target) + release.update_names(version, target): (root / name).write_bytes(b'fixture')
            release.verify_assets(root)
            signatures.assert_called_once_with(root)
            checksums = (root / 'SHA256SUMS').read_text().splitlines()
            self.assertEqual(len(checksums), 13)
            asset = root / release.asset_names(version, 'windows/amd64')[0]
            asset.write_bytes(b'')
            with self.assertRaises(ValueError): release.verify_assets(root)
            asset.unlink()
            with self.assertRaises(ValueError): release.verify_assets(root)

    def test_release_signature_failure_stops_verification(self):
        with tempfile.TemporaryDirectory() as temp, patch.object(release, 'verify_update_signatures', side_effect=ValueError('bad signature')):
            root = Path(temp); version = release.config()['version']
            for target in release.TARGETS:
                for name in release.asset_names(version, target) + release.update_names(version, target): (root / name).write_bytes(b'fixture')
            with self.assertRaisesRegex(ValueError, 'bad signature'): release.verify_assets(root)
            self.assertFalse((root / 'SHA256SUMS').exists())

    def test_published_releases_cannot_be_replaced(self):
        version = release.config()['version']
        env = {'GITHUB_REPOSITORY': 'example/nudilo', 'GH_TOKEN': 'fixture'}
        with patch.dict(release.os.environ, env), patch.object(release.urllib.request, 'urlopen', return_value=io.BytesIO(b'{"draft":false}')):
            with self.assertRaises(ValueError): release.guard('v' + version)
        with patch.dict(release.os.environ, env), patch.object(release.urllib.request, 'urlopen', return_value=io.BytesIO(b'{"draft":true}')):
            release.guard('v' + version)
        error = urllib.error.HTTPError('fixture', 403, 'Forbidden', {}, None)
        with patch.dict(release.os.environ, env), patch.object(release.urllib.request, 'urlopen', side_effect=error):
            with self.assertRaises(urllib.error.HTTPError): release.guard('v' + version)

    def test_binary_architectures_are_checked(self):
        elf = bytearray(20); elf[:6] = b'\x7fELF\x02\x01'; struct.pack_into('<H', elf, 18, 183)
        release.check_elf(elf, 'arm64')
        with self.assertRaises(ValueError): release.check_elf(elf, 'amd64')
        with self.assertRaises(ValueError): release.check_pe(b'MZ')
        with self.assertRaises(ValueError): release.check_elf(b'not a binary', 'amd64')

    def test_signed_mode_requires_credentials(self):
        with patch.dict(release.os.environ, {}, clear=True):
            with self.assertRaises(ValueError): release.signing_options({}, ['darwin/universal'], True)
            with self.assertRaises(ValueError): release.signing_options({}, ['windows/amd64'], True)
            cfg = {}; release.signing_options(cfg, ['darwin/universal'], False)
            self.assertEqual(cfg['macos']['signingIdentity'], '-')


if __name__ == '__main__': unittest.main()
