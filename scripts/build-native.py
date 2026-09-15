#!/usr/bin/env python3
"""Build the Rust static binding and zapp for one Windows/Linux target."""
import argparse
import datetime
import os
import sys
from pathlib import Path
import shutil
import subprocess

sys.path.insert(0, str(Path(__file__).resolve().parent))
import goreleaser_config as gr

ROOT = gr.ROOT
TARGETS = {
    "linux_amd64": "x86_64-unknown-linux-gnu",
    "linux_arm64": "aarch64-unknown-linux-gnu",
    "windows_amd64": "x86_64-pc-windows-gnullvm",
    "windows_arm64": "aarch64-pc-windows-gnullvm",
}


def binary_name(goos):
    return "zapp.exe" if goos == "windows" else "zapp"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("target", choices=TARGETS)
    parser.add_argument("--test", action="store_true", help="run Rust and Go tests on the target host")
    args = parser.parse_args()
    os.chdir(ROOT)
    target = TARGETS[args.target]
    env = os.environ.copy()
    env.update(zip(("GOOS", "GOARCH"), args.target.split("_")))
    env["CGO_ENABLED"] = "1"
    env["CARGO_TARGET_DIR"] = str(ROOT / "libcodesign/target")
    cc = env.get("CC", "gcc" if env["GOOS"] == "linux" else target.split("-")[0] + "-w64-mingw32-clang")
    env["CC"] = cc
    if env["GOOS"] == "windows":
        env.setdefault("AR", "llvm-ar")
    env["CARGO_TARGET_" + target.upper().replace("-", "_") + "_LINKER"] = cc
    subprocess.run(["rustup", "target", "add", target], check=True)
    cargo_args = ["--locked", "--release", "--manifest-path", "libcodesign/Cargo.toml", "--target", target]
    subprocess.run(["cargo", "build", *cargo_args], env=env, check=True)
    libdir = ROOT / "libcodesign/lib" / args.target
    libdir.mkdir(parents=True, exist_ok=True)
    shutil.copy2(ROOT / "libcodesign/target" / target / "release/libzapp_rcodesign.a", libdir)
    if args.test:
        # Only the test run pulls the crate's dev-dependencies in.
        subprocess.run(["cargo", "test", *cargo_args], env=env, check=True)
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
