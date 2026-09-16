#!/usr/bin/env python3
"""Download the prebuilt libcodesign static library for one target.

The Rust sources that used to live in this repository are now built and
released by github.com/ironpark/libcodesign, so a Windows or Linux build needs
a C toolchain but no Rust toolchain. scripts/libcodesign.json pins the release
and the SHA-256 of every archive; a download that does not match its pin is
rejected, and `--update <version>` is the only thing that rewrites the pins.

    python scripts/fetch_libcodesign.py linux_amd64
    python scripts/fetch_libcodesign.py --update v0.1.2
"""
import argparse
import hashlib
import json
import shutil
import sys
import tarfile
import tempfile
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PIN = Path(__file__).resolve().parent / "libcodesign.json"
# Where the archive is unpacked. pkg/signing/rcodesign/binding.go links
# <target>/lib/libzapp_rcodesign.a from here, so the two move together.
DESTINATION = ROOT / "third_party/libcodesign"
TARGETS = ("linux_amd64", "linux_arm64", "windows_amd64", "windows_arm64")


def config():
    return json.loads(PIN.read_text())


def url(pin, name):
    return f"https://github.com/{pin['repository']}/releases/download/{pin['version']}/{name}"


def archive_name(version, target):
    return f"libcodesign_{version.lstrip('v')}_{target}.tar.gz"


def download(address):
    print(f"fetching {address}", flush=True)
    with urllib.request.urlopen(address) as response:
        return response.read()


def fetch(target, force=False):
    """Unpack the pinned archive for target, returning its directory."""
    pin = config()
    digest = pin["archives"].get(target)
    if digest is None:
        raise SystemExit(f"{PIN.name} has no pin for {target}; run --update")
    directory = DESTINATION / target
    stamp = directory / ".version"
    if not force and stamp.is_file() and stamp.read_text() == pin["version"]:
        return directory

    name = archive_name(pin["version"], target)
    data = download(url(pin, name))
    actual = hashlib.sha256(data).hexdigest()
    if actual != digest:
        raise SystemExit(f"{name} is {actual}, not the pinned {digest}")

    # Replace the target's directory outright: a half-extracted or stale tree
    # would otherwise link into the next build.
    if directory.exists():
        shutil.rmtree(directory)
    directory.mkdir(parents=True)
    with tempfile.TemporaryDirectory() as tmp:
        bundle = Path(tmp) / name
        bundle.write_bytes(data)
        with tarfile.open(bundle) as tar:
            tar.extractall(directory, filter="data")
    stamp.write_text(pin["version"])
    print(f"{name} -> {directory}", flush=True)
    return directory


def update(version):
    """Repin every target to version, reading the release's own checksums."""
    pin = config()
    pin["version"] = version
    sums = download(url(pin, "SHA256SUMS")).decode()
    digests = dict(reversed(line.split()) for line in sums.splitlines() if line.strip())
    pin["archives"] = {}
    for target in TARGETS:
        name = archive_name(version, target)
        if name not in digests:
            raise SystemExit(f"{version} has no {name}")
        pin["archives"][target] = digests[name]
    PIN.write_text(json.dumps(pin, indent=2) + "\n")
    print(f"pinned {version}")


def main():
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("target", nargs="?", choices=TARGETS)
    parser.add_argument("--force", action="store_true", help="download even if the pinned version is already unpacked")
    parser.add_argument("--update", metavar="VERSION", help="repin every target to a libcodesign release")
    args = parser.parse_args()
    if args.update:
        update(args.update)
    elif args.target:
        print(fetch(args.target, args.force))
    else:
        parser.error("a target or --update is required")


if __name__ == "__main__":
    main()
