#!/usr/bin/env python3
"""Offline smoke tests for the pipe-compatible POSIX installer."""
import hashlib
import io
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest

INSTALLER = Path(__file__).with_name('install.sh').resolve()


class InstallTests(unittest.TestCase):
    def run_install(self, system='Linux', arch='x86_64', corrupt=False):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            tools = root / 'tools'
            tools.mkdir()
            install = root / 'install with spaces'
            install.mkdir()
            (install / 'zapp').write_bytes(b'old binary')
            archive = root / 'archive.tar.gz'
            with tarfile.open(archive, 'w:gz') as tar:
                data = b'#!/bin/sh\necho installed\n'
                entry = tarfile.TarInfo('zapp')
                entry.size = len(data)
                tar.addfile(entry, io.BytesIO(data))
            digest = hashlib.sha256(archive.read_bytes()).hexdigest()
            asset = f'zapp_{system}_{"arm64" if arch in ("arm64", "aarch64") else "x86_64"}.tar.gz'
            (root / 'checksums').write_text(f'{"0" * 64 if corrupt else digest}  {asset}\n')
            (tools / 'uname').write_text('#!/bin/sh\nif [ "$1" = -s ]; then echo "$TEST_OS"; else echo "$TEST_ARCH"; fi\n')
            (tools / 'curl').write_text('''#!/bin/sh
case "$*" in
  *releases/latest*) printf 'https://github.com/ironpark/zapp/releases/tag/v1.0.0-beta'; exit 0 ;;
esac
url=$2
out=$4
case "$url" in
  */"$TEST_ASSET") cp "$TEST_ROOT/archive.tar.gz" "$out" ;;
  */zapp_1.0.0-beta_checksums.txt) cp "$TEST_ROOT/checksums" "$out" ;;
  *) exit 22 ;;
esac
''')
            for tool in tools.iterdir():
                tool.chmod(0o755)
            env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ['PATH'],
                       ZAPP_INSTALL_DIR=str(install), ZAPP_VERSION='', TEST_OS=system,
                       TEST_ARCH=arch, TEST_ASSET=asset, TEST_ROOT=str(root))
            result = subprocess.run(['sh'], input=INSTALLER.read_text(), env=env,
                                    text=True, capture_output=True)
            if corrupt:
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('Checksum mismatch', result.stderr)
                self.assertEqual((install / 'zapp').read_bytes(), b'old binary')
            else:
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(subprocess.check_output([str(install / 'zapp')]), b'installed\n')
            self.assertEqual(sorted(p.name for p in install.iterdir()), ['zapp'])

    def test_platforms(self):
        for system, arch in [('Darwin', 'arm64'), ('Darwin', 'x86_64'),
                             ('Linux', 'aarch64'), ('Linux', 'x86_64')]:
            with self.subTest(system=system, arch=arch):
                self.run_install(system, arch)

    def test_checksum_failure_preserves_install(self):
        self.run_install(corrupt=True)


if __name__ == '__main__':
    unittest.main()
