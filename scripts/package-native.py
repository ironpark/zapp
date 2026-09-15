#!/usr/bin/env python3
"""Package one native zapp build, notices, and a SHA-256 checksum."""
import hashlib
from pathlib import Path
import sys
import tarfile
import zipfile

root = Path(__file__).resolve().parents[1]
goos, arch = sys.argv[1].split("_")
if goos not in ("windows", "linux") or arch not in ("amd64", "arm64"):
    raise SystemExit("expected linux/windows_amd64/arm64")
label = "x86_64" if arch == "amd64" else arch
name = f"zapp_{goos.title()}_{label}"
output = root / "release-assets"
output.mkdir(exist_ok=True)
files = [(root / "dist" / sys.argv[1] / ("zapp.exe" if goos == "windows" else "zapp"), "zapp.exe" if goos == "windows" else "zapp")]
for pattern in ("LICENSE*", "README*", "internal/thirdparty/text/LICENSE", "internal/thirdparty/text/PATENTS", "internal/thirdparty/text/README.md", "libcodesign/NOTICE", "libcodesign/Cargo.lock", "libcodesign/THIRD_PARTY_LICENSES.html"):
    files.extend((p, str(p.relative_to(root))) for p in root.glob(pattern) if p.is_file())
if not (root / "libcodesign/THIRD_PARTY_LICENSES.html").is_file():
    raise SystemExit("generate THIRD_PARTY_LICENSES.html with cargo-about before packaging")
if goos == "windows":
    archive = output / (name + ".zip")
    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as z:
        for src, dest in files:
            z.write(src, dest)
else:
    archive = output / (name + ".tar.gz")
    with tarfile.open(archive, "w:gz") as z:
        for src, dest in files:
            z.add(src, arcname=dest)
(output / (archive.name + ".sha256")).write_text(hashlib.sha256(archive.read_bytes()).hexdigest() + "  " + archive.name + "\n")
