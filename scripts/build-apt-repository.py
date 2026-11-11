#!/usr/bin/env python3
"""Create a signed stable APT repository from locally built release packages."""

import argparse
from datetime import datetime, timezone
from email.utils import format_datetime
import gzip
import hashlib
from pathlib import Path
import re
import shutil
import subprocess

PACKAGE = "plex-song-obs-overlay"
ARCHITECTURES = ("amd64", "arm64")


def build_repository(packages, output, signing_key):
    if not re.fullmatch(r"[0-9A-Fa-f]{40}|[0-9A-Fa-f]{64}", signing_key):
        raise ValueError("Supply the full fingerprint of a dedicated APT signing key")
    if output.exists() and any(output.iterdir()):
        raise ValueError("Output directory must be empty")
    output.mkdir(parents=True, exist_ok=True)
    records = {architecture: [] for architecture in ARCHITECTURES}
    for package in sorted(packages.glob("*.deb")):
        control = subprocess.check_output(["dpkg-deb", "--field", str(package)], text=True).strip()
        fields = dict(line.split(": ", 1) for line in control.splitlines()
                      if line and not line[0].isspace() and ": " in line)
        if fields.get("Package") != PACKAGE:
            raise ValueError(f"Unexpected package: {package}")
        architecture = fields.get("Architecture")
        if architecture not in records:
            raise ValueError(f"Unsupported architecture: {architecture}")
        version = fields.get("Version", "")
        if not re.fullmatch(r"\d+\.\d+\.\d+-1", version):
            raise ValueError(f"APT stable channel refuses non-stable version: {version}")
        relative = Path("pool/main/p") / PACKAGE / f"{PACKAGE}_{version}_{architecture}.deb"
        destination = output / relative
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(package, destination)
        content = destination.read_bytes()
        records[architecture].append(
            f"{control}\nFilename: {relative.as_posix()}\nSize: {len(content)}\n"
            f"SHA256: {hashlib.sha256(content).hexdigest()}\n"
            f"SHA512: {hashlib.sha512(content).hexdigest()}\n\n"
        )
    if not all(records.values()):
        raise ValueError("Both amd64 and arm64 stable packages are required")

    distribution = output / "dists/stable"
    indices = []
    for architecture, entries in records.items():
        directory = distribution / f"main/binary-{architecture}"
        directory.mkdir(parents=True)
        data = "".join(entries).encode()
        for name, content in [("Packages", data), ("Packages.gz", gzip.compress(data, mtime=0))]:
            path = directory / name
            path.write_bytes(content)
            indices.append(path)
            # Allow clients to fetch indices by digest while a new release is deploying.
            hashed = directory / "by-hash/SHA256" / hashlib.sha256(content).hexdigest()
            hashed.parent.mkdir(parents=True, exist_ok=True)
            hashed.write_bytes(content)
    release = distribution / "Release"
    metadata = (
        "Origin: Plex Song OBS Overlay\nLabel: Plex Song OBS Overlay\n"
        "Suite: stable\nCodename: stable\nArchitectures: amd64 arm64\n"
        "Components: main\nAcquire-By-Hash: yes\n"
        f"Date: {format_datetime(datetime.now(timezone.utc), usegmt=True)}\n"
        "Description: Stable Plex Song OBS Overlay packages\n"
    )
    for name, algorithm in [("SHA256", hashlib.sha256), ("SHA512", hashlib.sha512)]:
        metadata += name + ":\n"
        for index in indices:
            data = index.read_bytes()
            metadata += f" {algorithm(data).hexdigest()} {len(data)} {index.relative_to(distribution).as_posix()}\n"
    release.write_text(metadata)
    for options, destination in [(["--armor", "--detach-sign"], distribution / "Release.gpg"),
                                 (["--clearsign"], distribution / "InRelease")]:
        subprocess.run(["gpg", "--batch", "--yes", "--local-user", signing_key,
                        "--output", str(destination), *options, str(release)], check=True)
    public_key = subprocess.check_output(["gpg", "--armor", "--export", signing_key])
    if not public_key:
        raise ValueError("The APT public signing key could not be exported")
    (output / f"{PACKAGE}-archive-keyring.asc").write_bytes(public_key)
    (output / ".nojekyll").touch()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--packages", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--signing-key", required=True)
    args = parser.parse_args()
    build_repository(args.packages.resolve(), args.output.resolve(), args.signing_key)
