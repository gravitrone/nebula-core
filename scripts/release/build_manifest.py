#!/usr/bin/env python3
"""Build a Nebula release manifest with artifact checksums."""

from __future__ import annotations

import argparse
import hashlib
import json
from datetime import datetime, timezone
from pathlib import Path

PLATFORMS = (
    "darwin_arm64",
    "darwin_amd64",
    "linux_amd64",
    "linux_arm64",
)


def sha256(path: Path) -> str:
    """Return SHA-256 checksum for a file path."""
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def parse_args() -> argparse.Namespace:
    """Parse command line arguments."""
    parser = argparse.ArgumentParser(description="build nebula release manifest")
    parser.add_argument("--version", required=True)
    parser.add_argument("--channel", default="stable")
    parser.add_argument("--base-url", required=True, help="public artifact base URL")
    parser.add_argument("--out-dir", default="scripts/release/out")
    parser.add_argument(
        "--previous-manifest",
        default="",
        help="optional existing manifest.json to merge older releases",
    )
    return parser.parse_args()


def load_previous(path: Path) -> dict:
    """Load previous manifest if provided and valid."""
    if not path.exists():
        return {"latest": "", "versions": {}}

    payload = json.loads(path.read_text(encoding="utf-8"))
    releases = payload.get("releases") if isinstance(payload, dict) else None
    if not isinstance(releases, dict):
        return {"latest": "", "versions": {}}

    versions = releases.get("versions") if isinstance(releases.get("versions"), dict) else {}
    latest = releases.get("latest") if isinstance(releases.get("latest"), str) else ""
    return {"latest": latest, "versions": versions}


def main() -> None:
    """Build manifest and checksum files for a release version."""
    args = parse_args()
    out_dir = Path(args.out_dir).resolve()
    out_dir.mkdir(parents=True, exist_ok=True)

    previous_path = Path(args.previous_manifest).resolve() if args.previous_manifest else None
    previous = load_previous(previous_path) if previous_path else {"latest": "", "versions": {}}

    version = args.version
    release_entry: dict[str, object] = {"cli": {}, "compose": {}}

    checksums: dict[str, str] = {}

    for platform in PLATFORMS:
        artifact = out_dir / f"nebula-{version}-{platform}.tar.gz"
        if not artifact.exists():
            raise SystemExit(f"missing cli artifact: {artifact}")
        checksum = sha256(artifact)
        checksums[artifact.name] = checksum
        release_entry["cli"][platform] = {
            "url": f"{args.base_url.rstrip('/')}/{artifact.name}",
            "sha256": checksum,
        }

    compose_artifact = out_dir / f"nebula-runtime-{version}.tar.gz"
    if not compose_artifact.exists():
        raise SystemExit(f"missing runtime artifact: {compose_artifact}")

    compose_checksum = sha256(compose_artifact)
    checksums[compose_artifact.name] = compose_checksum
    release_entry["compose"] = {
        "url": f"{args.base_url.rstrip('/')}/{compose_artifact.name}",
        "sha256": compose_checksum,
    }

    checksum_file = out_dir / f"checksums-{version}.txt"
    checksum_lines = [f"{digest}  {name}" for name, digest in sorted(checksums.items())]
    checksum_file.write_text("\n".join(checksum_lines) + "\n", encoding="utf-8")

    versions = dict(previous["versions"])
    versions[version] = release_entry

    manifest = {
        "schema_version": 1,
        "channel": args.channel,
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "releases": {
            "latest": version,
            "versions": versions,
        },
    }

    manifest_path = out_dir / "manifest.json"
    manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=False) + "\n", encoding="utf-8")

    # Keep a previous snapshot for installer fallback.
    previous_copy = out_dir / "manifest.previous.json"
    if previous_path and previous_path.exists():
        previous_copy.write_text(previous_path.read_text(encoding="utf-8"), encoding="utf-8")
    else:
        previous_copy.write_text(json.dumps(manifest, indent=2, sort_keys=False) + "\n", encoding="utf-8")

    print(manifest_path)
    print(checksum_file)


if __name__ == "__main__":
    main()
