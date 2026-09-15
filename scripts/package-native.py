#!/usr/bin/env python3
"""Package one native zapp build into the archive GoReleaser would have made.

The layout -- contents, format and name -- comes from .goreleaser.yaml, so the
native half of a release cannot drift from the macOS half. Checksums are not
written here: .goreleaser.yaml lists these archives under checksum.extra_files,
so they land in the release's one checksums file.
"""
import sys
import tarfile
import zipfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import goreleaser_config as gr

root = gr.ROOT
goos, arch = sys.argv[1].split("_")
if goos not in ("windows", "linux") or arch not in ("amd64", "arm64"):
    raise SystemExit("expected linux/windows_amd64/arm64")

notices = root / "libcodesign/THIRD_PARTY_LICENSES.html"
if not notices.is_file():
    raise SystemExit("generate THIRD_PARTY_LICENSES.html with cargo-about before packaging")

output = root / "release-assets"
output.mkdir(exist_ok=True)
binary = "zapp.exe" if goos == "windows" else "zapp"
files = [(root / "dist" / sys.argv[1] / binary, binary)]
# The Rust notices are native-only: the macOS build does not link the library,
# so its archive has nothing to attribute and GoReleaser does not carry these.
patterns = gr.archive_files() + ["libcodesign/NOTICE", "libcodesign/Cargo.lock", str(notices.relative_to(root))]
for pattern in patterns:
    files.extend((p, str(p.relative_to(root))) for p in root.glob(pattern) if p.is_file())

name = gr.archive_name("zapp", goos, arch)
if gr.archive_format(goos) == "zip":
    archive = output / (name + ".zip")
    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as z:
        for src, dest in files:
            z.write(src, dest)
else:
    archive = output / (name + ".tar.gz")
    with tarfile.open(archive, "w:gz") as z:
        for src, dest in files:
            z.add(src, arcname=dest)
print(archive)
