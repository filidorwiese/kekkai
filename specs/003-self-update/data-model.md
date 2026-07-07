# Data Model: Self-Update Command

No persistent data. Transient values only.

## Release (remote)

| Field | Source | Notes |
|---|---|---|
| Tag | `tag_name` from GH releases/latest API | `vMAJOR.MINOR.PATCH` |
| TarballURL | derived: `.../download/<tag>/kekkai_<tag>_<goos>_<goarch>.tar.gz` | 404 = missing platform artifact |
| ChecksumManifest | `SHA256SUMS` asset | one line per artifact, `sha256sum` format |

## InstalledBinary (local)

| Field | Source | Notes |
|---|---|---|
| Version | `main.version` (ldflags) | `"dev"` when built without ldflags → refuse |
| Path | `os.Executable()` + `EvalSymlinks` | writability of file + parent dir gates the run (FR-008) |

## UpdateOutcome (exit states)

| Outcome | Condition | Exit | Filesystem effect |
|---|---|---|---|
| Updated | installed < latest, download + verify + rename OK | 0 | binary replaced atomically |
| UpToDate | installed == latest | 0 | none |
| Ahead | installed > latest | 0 | none |
| RefusedDev | version is dev/unversioned | 1 | none |
| ErrUnwritable | preflight write probe fails | 1 | none (before download) |
| ErrNetwork | API/download unreachable, rate limited | 1 | temp file cleaned |
| ErrNoArtifact | tarball 404 for platform | 1 | none |
| ErrChecksum | sha256 mismatch | 1 | temp file cleaned, binary untouched |

State order: dev guard → writability preflight → release lookup → compare → download → verify → extract → rename. Every failure short-circuits; the installed binary is only touched by the final rename.
