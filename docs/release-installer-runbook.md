# Nebula Release + Installer Runbook

## Scope

This runbook documents how to publish a stable Nebula release that can be installed with:

```bash
curl -fsSL https://nebula.gravitrone.com/install.sh | bash
```

It covers packaging, manifest publishing, rollback, and recovery.

## Local Data Layout

The installer manages data under `~/.nebula`:

- `~/.nebula/bin/nebula` - installed CLI binary
- `~/.nebula/runtime/releases/<version>/` - immutable runtime bundle per version
- `~/.nebula/runtime/current` - symlink to active runtime release
- `~/.nebula/data/postgres/` - persistent database data
- `~/.nebula/cache/` - downloaded artifacts and manifests
- `~/.nebula/logs/` - installer/runtime helper logs

## Manifest Contract

`manifest.json` schema:

```json
{
  "schema_version": 1,
  "channel": "stable",
  "generated_at": "2026-02-22T00:00:00+00:00",
  "releases": {
    "latest": "v1.0.0",
    "versions": {
      "v1.0.0": {
        "cli": {
          "darwin_arm64": {"url": "...", "sha256": "..."},
          "darwin_amd64": {"url": "...", "sha256": "..."},
          "linux_amd64": {"url": "...", "sha256": "..."},
          "linux_arm64": {"url": "...", "sha256": "..."}
        },
        "compose": {"url": "...", "sha256": "..."}
      }
    }
  }
}
```

A `manifest.previous.json` file must exist beside `manifest.json` for installer fallback.

## Release Steps

1. Tag release:
   - `git tag vX.Y.Z`
   - `git push origin vX.Y.Z`
2. GitHub `release` workflow runs:
   - builds CLI archives for all supported platforms
   - builds runtime bundle (compose + migrations + env template)
   - generates checksums + manifest files
   - uploads artifacts to release and workflow outputs
3. Publish artifacts + manifest files to channel location:
   - `https://nebula.gravitrone.com/channels/stable/`
4. Validate install on clean machine:
   - `NEBULA_CHANNEL=stable curl -fsSL https://nebula.gravitrone.com/install.sh | bash`

## Rollback Strategy

If a release is bad:

1. Keep bad artifacts in storage (do not delete immediately).
2. Set `releases.latest` in `manifest.json` back to previous healthy version.
3. Copy prior manifest to `manifest.previous.json`.
4. Purge CDN cache for both files.
5. Re-run smoke install and verify.

Installers can also pin explicit versions:

```bash
NEBULA_VERSION=vX.Y.Z curl -fsSL https://nebula.gravitrone.com/install.sh | bash
```

## Uninstall + Data Preservation Policy

- Safe uninstall keeps data by default:
  - stop stack: `docker compose --project-name nebula -f ~/.nebula/runtime/current/compose.yaml --env-file ~/.nebula/runtime/current/.env down`
  - remove runtime + binary: `rm -rf ~/.nebula/runtime ~/.nebula/bin/nebula ~/.nebula/cache`
- Persistent data remains in `~/.nebula/data/postgres` unless user explicitly deletes it.
- Full wipe (destructive): `rm -rf ~/.nebula`

## Failure Recovery

### Installer fails at manifest fetch

- Check `manifest.json` and `manifest.previous.json` are publicly reachable.
- Verify TLS/certificate chain at CDN edge.

### Installer fails checksum validation

- Regenerate checksums with `scripts/release/build_manifest.py`.
- Ensure uploaded artifact bytes match the checksummed artifacts.

### Runtime starts but Postgres is unhealthy

- Inspect:
  - `docker compose --project-name nebula -f ~/.nebula/runtime/current/compose.yaml --env-file ~/.nebula/runtime/current/.env logs postgres`
- Validate `NEBULA_DATA_DIR` exists and is writable.

### Binary installed but command not found

- Add to shell profile:
  - `export PATH="$HOME/.nebula/bin:$PATH"`

## CI/CD Guardrails

- `ci.yml` enforces server tests, CLI tests, lint on PR and protected branches.
- `release.yml` only runs from version tags.
- `smoke-install.yml` validates installer against local channel artifacts.

## Explicit External Dependencies

These pieces require infra/org access outside this repo:

- DNS + CDN routing for `nebula.gravitrone.com`
- Bucket/object permissions for channel files
- GitHub org secrets for release publish credentials
- GHCR org permissions and signing keys
- TLS cert lifecycle + CDN cache controls
- Apple notarization/signing credentials for macOS trust chain
