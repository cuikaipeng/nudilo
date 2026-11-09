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

    def test_github_api_only_tolerates_explicit_missing_reads(self):
        env = {'GITHUB_REPOSITORY': 'example/nudilo', 'GH_TOKEN': 'fixture'}
        with patch.dict(release.os.environ, env), patch.object(release.urllib.request, 'urlopen', return_value=io.BytesIO(b'{"id":1}')):
            self.assertEqual(release.github_api('releases/1'), {'id': 1})
        for code in (403, 404, 500):
            error = urllib.error.HTTPError('fixture', code, 'Fixture error', {}, None)
            with self.subTest(code=code), patch.dict(release.os.environ, env), patch.object(release.urllib.request, 'urlopen', side_effect=error):
                with self.assertRaises(urllib.error.HTTPError): release.github_api('releases')
                with self.assertRaises(urllib.error.HTTPError): release.github_api('git/refs', method='POST', missing_ok=True)
                if code == 404:
                    self.assertIsNone(release.github_api('git/ref/tags/v1.0.0', missing_ok=True))
                else:
                    with self.assertRaises(urllib.error.HTTPError): release.github_api('git/ref/tags/v1.0.0', missing_ok=True)

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


class PublicationTests(unittest.TestCase):
    def setUp(self):
        self.cfg = release.config()
        self.tag = 'v' + self.cfg['version']
        self.commit = 'a' * 40
        self.releases, self.assets, self.writes = [], [], []
        self.reference = None
        self.annotated = {}
        env = {
            'GITHUB_REPOSITORY': self.cfg['updates']['github'], 'GH_TOKEN': 'fixture',
            'GITHUB_EVENT_NAME': 'push', 'GITHUB_REF': 'refs/heads/main', 'GITHUB_SHA': self.commit,
        }
        environment = patch.dict(release.os.environ, env, clear=True)
        environment.start(); self.addCleanup(environment.stop)
        api = patch.object(release, 'github_api', side_effect=self.github)
        self.api = api.start(); self.addCleanup(api.stop)

    def github(self, path, *, method='GET', data=None, missing_ok=False):
        if method != 'GET':
            self.writes.append((method, path, data))
            if method == 'POST' and path == 'git/refs':
                self.reference = {'type': 'commit', 'sha': data['sha']}
                return {'ref': data['ref'], 'object': self.reference}
            if method == 'PATCH' and path == 'releases/1':
                return {'tag_name': self.tag, 'draft': False, 'prerelease': False, 'html_url': 'https://example.com/release'}
        elif path.startswith(('releases?', 'releases/1/assets?')):
            items = self.assets if '/assets?' in path else self.releases
            page = int(path.rsplit('=', 1)[1])
            return items[(page - 1) * 100:page * 100]
        elif path == 'git/ref/tags/' + self.tag:
            return {'object': self.reference} if self.reference else None
        elif path.startswith('git/tags/'):
            return {'object': self.annotated[path.removeprefix('git/tags/')]}
        raise AssertionError(f'Unexpected GitHub request: {method} {path}')

    def draft_assets(self):
        temp = tempfile.TemporaryDirectory(); self.addCleanup(temp.cleanup)
        directory = Path(temp.name)
        signatures = patch.object(release, 'verify_update_signatures')
        self.signatures = signatures.start(); self.addCleanup(signatures.stop)
        self.reference = {'type': 'commit', 'sha': self.commit}
        self.releases = [{'id': 1, 'tag_name': self.tag, 'draft': True, 'prerelease': False}]
        for target in release.TARGETS:
            for name in release.asset_names(self.cfg['version'], target) + release.update_names(self.cfg['version'], target):
                (directory / name).write_bytes(b'fixture')
        release.verify_assets(directory)
        self.assets = [{'name': p.name, 'state': 'uploaded', 'size': p.stat().st_size, 'digest': 'sha256:' + release.digest(p)}
                       for p in directory.iterdir()]
        return directory

    def test_prepare_creates_tag_at_this_main_commit(self):
        release.prepare_tag(self.tag, self.commit)
        self.assertEqual(self.reference, {'type': 'commit', 'sha': self.commit})
        self.assertEqual(self.writes, [('POST', 'git/refs', {'ref': 'refs/tags/' + self.tag, 'sha': self.commit})])

    def test_same_commit_tag_and_draft_can_be_retried(self):
        self.reference = {'type': 'tag', 'sha': 'b' * 40}
        self.annotated['b' * 40] = {'type': 'commit', 'sha': self.commit}
        release.prepare_tag(self.tag, self.commit)  # Interrupted before draft creation.
        self.releases = [{'id': 1, 'tag_name': self.tag, 'draft': True, 'prerelease': False}]
        release.prepare_tag(self.tag, self.commit)  # Interrupted during upload.
        self.assertEqual(self.writes, [])

    def test_existing_tag_for_different_commit_is_never_moved(self):
        self.reference = {'type': 'commit', 'sha': 'b' * 40}
        with self.assertRaisesRegex(ValueError, 'another commit'): release.prepare_tag(self.tag, self.commit)
        self.assertEqual(self.writes, [])

    def test_pr_guard_requires_an_unused_tag(self):
        self.reference = {'type': 'commit', 'sha': self.commit}
        with self.assertRaisesRegex(ValueError, 'already exists'): release.guard(self.tag)

    def test_published_version_cannot_be_overwritten(self):
        self.releases = [{'tag_name': self.tag, 'draft': False, 'prerelease': False}]
        with self.assertRaisesRegex(ValueError, 'already published'): release.prepare_tag(self.tag, self.commit)
        self.assertEqual(self.writes, [])

    def test_version_order_is_numeric_and_cannot_regress(self):
        cfg = dict(self.cfg, version='1.10.0')
        self.tag = 'v1.10.0'
        self.releases = [{'tag_name': 'v1.9.9', 'draft': False, 'prerelease': False}]
        with patch.object(release, 'config', return_value=cfg):
            release.guard(self.tag)
            self.releases.append({'tag_name': 'v1.11.0', 'draft': False, 'prerelease': False})
            with self.assertRaisesRegex(ValueError, 'newer than published v1.11.0'): release.guard(self.tag)

    def test_guards_consider_older_pages_and_ignore_unpublished_versions(self):
        self.releases = [{'tag_name': f'v99.0.{index}', 'draft': True, 'prerelease': False} for index in range(100)]
        self.releases.append({'tag_name': 'v99.1.0', 'draft': False, 'prerelease': True})
        release.guard(self.tag)
        self.releases.append({'tag_name': 'v99.2.0', 'draft': False, 'prerelease': False})
        with self.assertRaisesRegex(ValueError, 'newer than published v99.2.0'): release.guard(self.tag)

    def test_draft_without_matching_tag_is_rejected(self):
        self.releases = [{'tag_name': self.tag, 'draft': True, 'prerelease': False}]
        with self.assertRaisesRegex(ValueError, 'same commit'): release.prepare_tag(self.tag, self.commit)
        self.assertEqual(self.writes, [])

    def test_only_the_main_push_commit_can_change_releases(self):
        cases = [
            {'GITHUB_REF': 'refs/heads/dev'}, {'GITHUB_REF': 'refs/tags/' + self.tag},
            {'GITHUB_EVENT_NAME': 'pull_request'}, {'GITHUB_EVENT_NAME': 'workflow_dispatch'},
            {'GITHUB_SHA': 'b' * 40}, {'GITHUB_REPOSITORY': 'example/fork'},
        ]
        for env in cases:
            with self.subTest(env=env), patch.dict(release.os.environ, env):
                with self.assertRaises(ValueError): release.prepare_tag(self.tag, self.commit)
                with self.assertRaises(ValueError): release.publish(self.tag, self.commit, Path('unused'))
        self.api.assert_not_called()

    def test_complete_verified_upload_is_published_as_latest(self):
        directory = self.draft_assets()
        release.publish(self.tag, self.commit, directory)
        self.assertEqual(self.writes, [('PATCH', 'releases/1', {'draft': False, 'prerelease': False, 'make_latest': 'true'})])

    def test_missing_extra_or_duplicate_uploads_remain_drafts(self):
        directory = self.draft_assets()
        original = self.assets
        for assets in (original[:-1], original + [dict(original[0], name='extra')], original[:-1] + [original[0]]):
            with self.subTest(names=[a['name'] for a in assets]):
                self.assets = assets
                with self.assertRaisesRegex(ValueError, 'do not match'): release.publish(self.tag, self.commit, directory)
        self.assertEqual(self.writes, [])

    def test_incomplete_or_tampered_uploads_remain_drafts(self):
        directory = self.draft_assets()
        original = dict(self.assets[0])
        for changed in ({'state': 'starter'}, {'size': original['size'] + 1}, {'digest': 'sha256:' + '0' * 64}, {'digest': None}):
            with self.subTest(changed=changed):
                self.assets[0] = dict(original, **changed)
                with self.assertRaisesRegex(ValueError, 'size/SHA-256'): release.publish(self.tag, self.commit, directory)
        self.assertEqual(self.writes, [])

    def test_signature_failure_stops_publication(self):
        directory = self.draft_assets()
        self.signatures.side_effect = ValueError('bad signature')
        with self.assertRaisesRegex(ValueError, 'bad signature'): release.publish(self.tag, self.commit, directory)
        self.api.assert_not_called()

    def test_final_guard_stops_publication_if_version_became_public(self):
        directory = self.draft_assets()
        self.releases[0]['draft'] = False
        with self.assertRaisesRegex(ValueError, 'already published'): release.publish(self.tag, self.commit, directory)
        self.assertEqual(self.writes, [])


if __name__ == '__main__': unittest.main()
