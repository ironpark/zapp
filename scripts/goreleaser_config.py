"""Read the parts of .goreleaser.yaml the native build has to agree with.

Windows and Linux binaries link a Rust static library through cgo, so they are
built on their own runners rather than by GoReleaser. GoReleaser's OSS
distribution cannot adopt a binary it did not build itself -- `builder:
prebuilt` is Pro-only -- so the archive for those targets is assembled by
scripts/package-native.py instead.

That split is fine as long as there is still only one definition of what a
release looks like. These helpers make .goreleaser.yaml that definition, so the
archive contents, format, name and version stamping cannot drift between the
macOS half of a release and the native half.
"""
from pathlib import Path

try:
    import yaml
except ModuleNotFoundError as exc:  # pragma: no cover - environment problem
    raise SystemExit("PyYAML is required to read .goreleaser.yaml: pip install pyyaml") from exc

ROOT = Path(__file__).resolve().parents[1]

# The archive name is a Go template, which is not worth an evaluator here. The
# scheme below implements this exact template; if it is ever edited, packaging
# fails loudly rather than quietly producing differently named assets.
NAME_TEMPLATE = (
    '{{ .ProjectName }}_ {{- title .Os }}_ {{- if eq .Arch "amd64" }}x86_64 '
    '{{- else if eq .Arch "386" }}i386 {{- else }}{{ .Arch }}{{ end }} '
    "{{- if .Arm }}v{{ .Arm }}{{ end }}"
)
ARCH_NAMES = {"amd64": "x86_64", "386": "i386"}


def load():
    return yaml.safe_load((ROOT / ".goreleaser.yaml").read_text())


def archive_name(project, goos, arch):
    """Name one archive the way GoReleaser names its own."""
    config = load()["archives"][0]
    if config["name_template"].split() != NAME_TEMPLATE.split():
        raise SystemExit(
            "archives.name_template in .goreleaser.yaml no longer matches the scheme "
            "scripts/goreleaser_config.py implements; update both together"
        )
    return f"{project}_{goos.title()}_{ARCH_NAMES.get(arch, arch)}"


def archive_format(goos):
    """Report tar.gz or zip for goos, honouring format_overrides."""
    config = load()["archives"][0]
    for override in config.get("format_overrides", []):
        if override["goos"] == goos:
            return _format(override)
    return _format(config)


def _format(section):
    """Read a formats list, falling back to the deprecated scalar form."""
    formats = section.get("formats")
    if formats:
        return formats[0]
    return section.get("format", "tar.gz")


def archive_files():
    """The globs every release archive carries, relative to the repo root."""
    return list(load()["archives"][0]["files"])


def ldflags(version, commit, date):
    """GoReleaser's ldflags, with the fields it would have templated in."""
    flags = load()["builds"][0]["ldflags"]
    for field, value in (("Version", version), ("Commit", commit), ("Date", date)):
        flags = flags.replace("{{." + field + "}}", value)
    if "{{" in flags:
        raise SystemExit(f"unsupported template field in builds.ldflags: {flags}")
    return flags
