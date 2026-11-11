#!/usr/bin/env python3
"""Build static Linux binaries, Debian packages, and portable installation trees."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]
PACKAGE = "plex-song-obs-overlay"


def package_version(tag):
    if not re.fullmatch(r"v?\d+\.\d+\.\d+(?:-rc\.[1-9]\d*)?", tag):
        raise ValueError(f"Unsupported version: {tag!r}")
    return tag.removeprefix("v").replace("-rc.", "~rc.")


def install_tree(binary, destination):
    files = {
        binary: (f"usr/bin/{PACKAGE}", 0o755),
        ROOT / "packaging/linux/plex-song-obs-overlay.desktop":
            (f"usr/share/applications/{PACKAGE}.desktop", 0o644),
        ROOT / "assets/plex-song-obs-overlay.png":
            (f"usr/share/pixmaps/{PACKAGE}.png", 0o644),
        ROOT / "LICENSE": (f"usr/share/licenses/{PACKAGE}/LICENSE", 0o644),
        ROOT / "README.md": (f"usr/share/doc/{PACKAGE}/README.md", 0o644),
    }
    for source, (relative, mode) in files.items():
        target = destination / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, target)
        target.chmod(mode)


def build(architectures, output, portable_only=False):
    tag = json.loads((ROOT / "version.json").read_text())["version"]
    version = package_version(tag)
    output.mkdir(parents=True, exist_ok=True)
    artifacts = []
    epoch = int(os.environ.get("SOURCE_DATE_EPOCH", str(int((ROOT / "version.json").stat().st_mtime))))
    for architecture in architectures:
        with tempfile.TemporaryDirectory(prefix="plex-package-") as temporary:
            work = Path(temporary)
            binary = work / PACKAGE
            subprocess.run(
                ["go", "build", "-mod=readonly", "-trimpath", "-buildvcs=false",
                 "-ldflags=-s -w", "-o", str(binary), "."],
                cwd=ROOT, check=True,
                env={**os.environ, "GOOS": "linux", "GOARCH": architecture,
                     "CGO_ENABLED": "0"},
            )
            tree = work / "root"
            install_tree(binary, tree)
            archive = output / f"{PACKAGE}_{version}_linux_{architecture}.tar.gz"
            # A regular installation tree; no post-install scripts or root service.
            with tarfile.open(archive, "w:gz") as bundle:
                for path in sorted(tree.rglob("*")):
                    info = bundle.gettarinfo(path, arcname=str(path.relative_to(tree)))
                    info.uid = info.gid = 0
                    info.uname = info.gname = "root"
                    info.mtime = epoch
                    if path.is_file():
                        with path.open("rb") as source:
                            bundle.addfile(info, source)
                    else:
                        bundle.addfile(info)
            artifacts.append(archive)
            if portable_only:
                continue
            metadata = tree / "DEBIAN"
            metadata.mkdir()
            size = sum(p.stat().st_size for p in tree.rglob("*") if p.is_file())
            (metadata / "control").write_text(
                f"Package: {PACKAGE}\nVersion: {version}-1\nArchitecture: {architecture}\n"
                "Section: sound\nPriority: optional\n"
                "Maintainer: SpawnInsane <spawninsane@gmail.com>\n"
                f"Installed-Size: {(size + 1023) // 1024}\n"
                "Depends: ca-certificates, xdg-utils\n"
                "Suggests: obs-studio\n"
                "Homepage: https://github.com/SpawnInsane/Plex-Song-OBS-Overlay\n"
                "Description: Show your current Plex song in OBS\n"
                " A local desktop application with browser-based controls and a transparent\n"
                " OBS Browser Source showing Plex music playback.\n"
            )
            # Debian's standard copyright location, in addition to the shared tree.
            shutil.copyfile(ROOT / "LICENSE", tree / f"usr/share/doc/{PACKAGE}/copyright")
            deb = output / f"{PACKAGE}_{version}-1_{architecture}.deb"
            subprocess.run(["dpkg-deb", "--root-owner-group", "-Zxz", "--build",
                            str(tree), str(deb)], check=True,
                           env={**os.environ, "SOURCE_DATE_EPOCH": str(epoch)})
            artifacts.append(deb)
    for artifact in artifacts:
        digest = hashlib.sha256(artifact.read_bytes()).hexdigest()
        artifact.with_name(artifact.name + ".sha256").write_text(f"{digest}  {artifact.name}\n")
        print(artifact)
    return artifacts


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--arch", choices=["amd64", "arm64"], action="append")
    parser.add_argument("--output", type=Path, default=ROOT / "dist")
    parser.add_argument("--portable-only", action="store_true", help="Do not require dpkg-deb")
    args = parser.parse_args()
    build(args.arch or ["amd64", "arm64"], args.output.resolve(), args.portable_only)
