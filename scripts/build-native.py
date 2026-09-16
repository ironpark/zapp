#!/usr/bin/env python3
"""Build zapp for one Windows/Linux target against the prebuilt libcodesign."""
import argparse
import datetime
import os
import sys
from pathlib import Path
import subprocess

sys.path.insert(0, str(Path(__file__).resolve().parent))
import fetch_libcodesign
import goreleaser_config as gr

ROOT = gr.ROOT
TARGETS = fetch_libcodesign.TARGETS
# LLVM MinGW names its compilers after the architecture, not GOARCH.
MINGW_ARCHES = {"amd64": "x86_64", "arm64": "aarch64"}


def binary_name(goos):
    return "zapp.exe" if goos == "windows" else "zapp"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("target", choices=TARGETS)
    parser.add_argument("--test", action="store_true", help="run the Go tests on the target host")
    args = parser.parse_args()
    os.chdir(ROOT)
    env = os.environ.copy()
    env.update(zip(("GOOS", "GOARCH"), args.target.split("_")))
    env["CGO_ENABLED"] = "1"
    # The Windows archives are built with LLVM MinGW, so linking them needs the
    # same toolchain rather than the host's MSVC.
    env["CC"] = env.get("CC", "gcc" if env["GOOS"] == "linux"
                        else MINGW_ARCHES[env["GOARCH"]] + "-w64-mingw32-clang")
    fetch_libcodesign.fetch(args.target)
    if args.test:
        subprocess.run(["go", "test", "-count=1", "./pkg/signing/..."], env=env, check=True)
        if env["GOOS"] == "windows":
            subprocess.run(["go", "test", "-count=1", "-run", "^TestWindowsPayloadMetadata$", "./pkg/macpkg"], env=env, check=True)
    output = ROOT / "dist" / args.target / binary_name(env["GOOS"])
    output.parent.mkdir(parents=True, exist_ok=True)
    commit = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
    version = os.environ.get("ZAPP_VERSION", "dev")
    date = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    ldflags = gr.ldflags(version, commit, date)
    subprocess.run(["go", "build", "-trimpath", "-ldflags", ldflags, "-o", str(output), "./cmd/zapp"], env=env, check=True)
    if env["GOOS"] == "windows":
        imports = subprocess.check_output(["llvm-readobj", "--coff-imports", str(output)], text=True).lower()
        for dependency in ("libunwind.dll", "libc++.dll", "libwinpthread-1.dll", "rcodesign.dll"):
            if dependency in imports:
                raise SystemExit(f"unexpected runtime dependency: {dependency}")
    print(output)


if __name__ == "__main__":
    main()
