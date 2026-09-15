#!/usr/bin/env python3
"""Build the Rust static binding and zapp for one Windows/Linux target."""
import argparse
import datetime
import os
from pathlib import Path
import shutil
import subprocess

ROOT = Path(__file__).resolve().parents[1]
TARGETS = {
    "linux_amd64": "x86_64-unknown-linux-gnu",
    "linux_arm64": "aarch64-unknown-linux-gnu",
    "windows_amd64": "x86_64-pc-windows-gnullvm",
    "windows_arm64": "aarch64-pc-windows-gnullvm",
}


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
    cargo = ["cargo", "build", "--locked", "--release", "--manifest-path", "libcodesign/Cargo.toml", "--target", target]
    subprocess.run(cargo, env=env, check=True)
    libdir = ROOT / "libcodesign/lib" / args.target
    libdir.mkdir(parents=True, exist_ok=True)
    shutil.copy2(ROOT / "libcodesign/target" / target / "release/libzapp_rcodesign.a", libdir)
    if args.test:
        cargo[1] = "test"
        subprocess.run(cargo, env=env, check=True)
        subprocess.run(["go", "test", "-count=1", "./pkg/signing/..."], env=env, check=True)
        if env["GOOS"] == "windows":
            subprocess.run(["go", "test", "-count=1", "-run", "^TestWindowsPayloadMetadata$", "./pkg/macpkg"], env=env, check=True)
    output = ROOT / "dist" / args.target / ("zapp.exe" if env["GOOS"] == "windows" else "zapp")
    output.parent.mkdir(parents=True, exist_ok=True)
    commit = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
    version = os.environ.get("ZAPP_VERSION", "dev")
    date = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    ldflags = f"-s -w -X github.com/ironpark/zapp/cmd/info.Version={version} -X github.com/ironpark/zapp/cmd/info.Commit={commit} -X github.com/ironpark/zapp/cmd/info.BuildDate={date}"
    subprocess.run(["go", "build", "-trimpath", "-ldflags", ldflags, "-o", str(output), "."], env=env, check=True)
    if env["GOOS"] == "windows":
        imports = subprocess.check_output(["llvm-readobj", "--coff-imports", str(output)], text=True).lower()
        for dependency in ("libunwind.dll", "libc++.dll", "libwinpthread-1.dll", "rcodesign.dll"):
            if dependency in imports:
                raise SystemExit(f"unexpected runtime dependency: {dependency}")
    print(output)


if __name__ == "__main__":
    main()
