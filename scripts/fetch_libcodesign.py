#!/usr/bin/env python3
"""Refresh the bundled libcodesign static library for one target.

Prebuilt libraries from github.com/ironpark/libcodesign are checked into
pkg/signing/rcodesign/lib, a Go module of its own that only cgo builds for
Linux and Windows download. Consumers need a C toolchain but no Rust toolchain
or download step. scripts/libcodesign.json pins the release
and the SHA-256 of every archive; a download that does not match its pin is
rejected, and `--update <version>` is the only thing that rewrites the pins.

    python scripts/fetch_libcodesign.py linux_amd64
    python scripts/fetch_libcodesign.py --update v0.1.2
"""
import argparse
import hashlib
import io
import json
import tarfile
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PIN = Path(__file__).resolve().parent / "libcodesign.json"
# The binding compiles against the header; the archives and the notices that
# must travel with them go to the lib module, which links them.
BINDING = ROOT / "pkg/signing/rcodesign"
LIBRARIES = BINDING / "lib"
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


def fetch(target):
    """Verify the pinned archive and copy only build inputs and license notices."""
    pin = config()
    digest = pin["archives"].get(target)
    if digest is None:
        raise SystemExit(f"{PIN.name} has no pin for {target}; run --update")
    name = archive_name(pin["version"], target)
    data = download(url(pin, name))
    actual = hashlib.sha256(data).hexdigest()
    if actual != digest:
        raise SystemExit(f"{name} is {actual}, not the pinned {digest}")

    goos = target.split("_")[0]
    files = {
        "lib/libzapp_rcodesign.a": LIBRARIES / f"{target}.a",
        "include/zapp_rcodesign.h": BINDING / "zapp_rcodesign.h",
        "LICENSE": LIBRARIES / "LICENSE",
        "NOTICE": LIBRARIES / "NOTICE",
        "THIRD_PARTY_LICENSES.html": LIBRARIES / "licenses" / f"{goos}.html",
    }
    # Validate every required member before changing any bundled file. Never
    # extract the upstream README, Cargo.lock, or arbitrary archive paths.
    payloads = {}
    with tarfile.open(fileobj=io.BytesIO(data)) as tar:
        for source, destination in files.items():
            member = tar.getmember(source)
            if not member.isfile():
                raise SystemExit(f"{name}: {source} is not a regular file")
            payload = tar.extractfile(member).read()
            if destination.suffix != ".a":
                payload = payload.replace(b"\r\n", b"\n")
            payloads[destination] = payload
    for path, payload in payloads.items():
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(payload)
    library = LIBRARIES / f"{target}.a"
    print(f"{name} -> {library}", flush=True)
    return library


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
    parser.add_argument("--update", metavar="VERSION", help="repin every target to a libcodesign release")
    args = parser.parse_args()
    if args.update:
        update(args.update)
    elif args.target:
        print(fetch(args.target))
    else:
        parser.error("a target or --update is required")


if __name__ == "__main__":
    main()
