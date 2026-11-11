"""Exercise package layout, version ordering, and a signed APT repository."""

import gzip
import hashlib
import importlib.util
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]


def load_script(filename):
    spec = importlib.util.spec_from_file_location(filename, ROOT / "scripts" / filename)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


packages = load_script("build-linux-packages.py")
repository = load_script("build-apt-repository.py")


class LinuxPackageTests(unittest.TestCase):
    def test_release_candidate_sorts_before_stable(self):
        self.assertEqual(packages.package_version("v1.2.3-rc.4"), "1.2.3~rc.4")
        self.assertEqual(packages.package_version("v1.2.3"), "1.2.3")
        if shutil.which("dpkg"):
            subprocess.run(["dpkg", "--compare-versions", "1.2.3~rc.4-1", "lt", "1.2.3-1"], check=True)

    def test_version_rejects_paths_and_unexpected_tags(self):
        for version in ["../../outside", "1.2", "v1.2.3\n", "v1.2.3-rc.0", "1.2.3-beta"]:
            with self.subTest(version=version), self.assertRaises(ValueError):
                packages.package_version(version)

    def test_installation_has_executable_launcher_icon_and_license(self):
        with tempfile.TemporaryDirectory() as temporary:
            work = Path(temporary)
            binary = work / "binary"
            binary.write_bytes(b"test executable")
            tree = work / "root"
            packages.install_tree(binary, tree)
            executable = tree / "usr/bin/plex-song-obs-overlay"
            self.assertEqual(executable.stat().st_mode & 0o777, 0o755)
            launcher = tree / "usr/share/applications/plex-song-obs-overlay.desktop"
            self.assertIn("Exec=plex-song-obs-overlay\n", launcher.read_text())
            self.assertTrue((tree / "usr/share/pixmaps/plex-song-obs-overlay.png").is_file())
            self.assertEqual((tree / "usr/share/licenses/plex-song-obs-overlay/LICENSE").read_bytes(),
                             (ROOT / "LICENSE").read_bytes())


@unittest.skipUnless(shutil.which("dpkg-deb") and shutil.which("gpg"), "dpkg-deb and gpg required")
class AptRepositoryTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.work = Path(self.temporary.name)
        self.debs = self.work / "debs"
        self.debs.mkdir()
        gnupg = self.work / "gnupg"
        gnupg.mkdir(mode=0o700)
        environment = patch.dict(os.environ, {"GNUPGHOME": str(gnupg)})
        environment.start()
        self.addCleanup(environment.stop)
        self.addCleanup(lambda: subprocess.run(["gpgconf", "--kill", "gpg-agent"], check=False))
        subprocess.run(["gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase", "",
                        "--quick-generate-key", "Packaging test <test@example.invalid>",
                        "ed25519", "sign", "1d"], check=True, capture_output=True)
        keys = subprocess.check_output(["gpg", "--batch", "--with-colons", "--list-keys"], text=True)
        self.fingerprint = next(line.split(":")[9] for line in keys.splitlines() if line.startswith("fpr:"))

    def make_deb(self, architecture, version="1.2.3-1"):
        tree = self.work / architecture
        (tree / "DEBIAN").mkdir(parents=True)
        (tree / "DEBIAN/control").write_text(
            f"Package: plex-song-obs-overlay\nVersion: {version}\nArchitecture: {architecture}\n"
            "Maintainer: Test <test@example.invalid>\nDescription: Packaging test\n")
        subprocess.run(["dpkg-deb", "--root-owner-group", "--build", str(tree),
                        str(self.debs / f"package-{architecture}.deb")], check=True, capture_output=True)

    def test_signed_indices_reference_both_architectures_and_correct_hashes(self):
        for architecture in ("amd64", "arm64"):
            self.make_deb(architecture)
        site = self.work / "site"
        repository.build_repository(self.debs, site, self.fingerprint)
        release = site / "dists/stable/Release"
        subprocess.run(["gpg", "--batch", "--verify", str(release.with_name("Release.gpg")),
                        str(release)], check=True, capture_output=True)
        subprocess.run(["gpg", "--batch", "--verify", str(release.with_name("InRelease"))],
                       check=True, capture_output=True)
        for architecture in ("amd64", "arm64"):
            directory = site / f"dists/stable/main/binary-{architecture}"
            index = directory / "Packages"
            data = index.read_bytes()
            fields = dict(line.split(": ", 1) for line in data.decode().splitlines() if ": " in line)
            package = site / fields["Filename"]
            self.assertEqual(fields["SHA256"], hashlib.sha256(package.read_bytes()).hexdigest())
            self.assertEqual(int(fields["Size"]), package.stat().st_size)
            self.assertEqual(gzip.decompress((directory / "Packages.gz").read_bytes()), data)
            digest = hashlib.sha256(data).hexdigest()
            self.assertEqual((directory / "by-hash/SHA256" / digest).read_bytes(), data)
            self.assertIn(f" {digest} {len(data)} main/binary-{architecture}/Packages", release.read_text())
        self.assertIn(b"BEGIN PGP PUBLIC KEY BLOCK", (site / "plex-song-obs-overlay-archive-keyring.asc").read_bytes())
        self.assertFalse(any("PRIVATE KEY" in p.read_text(errors="ignore") for p in site.rglob("*") if p.is_file()))

    def test_stable_repository_rejects_prereleases(self):
        self.make_deb("amd64", "1.2.3~rc.1-1")
        with self.assertRaisesRegex(ValueError, "non-stable version"):
            repository.build_repository(self.debs, self.work / "site", self.fingerprint)

    def test_repository_requires_all_architectures(self):
        self.make_deb("amd64")
        with self.assertRaisesRegex(ValueError, "Both amd64 and arm64"):
            repository.build_repository(self.debs, self.work / "site", self.fingerprint)


if __name__ == "__main__":
    unittest.main()
