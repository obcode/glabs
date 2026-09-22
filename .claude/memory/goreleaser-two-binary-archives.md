---
name: goreleaser-two-binary-archives
description: "glabs releases attach binaries only if goreleaser archives are split per build (glabs all-platforms, glabs-web linux-only)"
metadata: 
  node_type: memory
  type: project
  originSessionId: 7a79cd9d-55ad-46e6-a6df-8e5a59aaf26e
---

The glabs repo builds two binaries via goreleaser: `glabs` (linux/windows/darwin) and `glabs-web` (linux only). goreleaser v2 bundles every binary of a build into one archive **per platform**, so if both share the default archive, linux archives hold two binaries and macOS/Windows hold one — goreleaser rejects the mismatch ("archive has different count of binaries for each platform") and the release job fails at the archive step. The GitHub Release/tag is still created by go-semantic-release *before* the goreleaser hook runs, so the failure is silent: a tag exists with **zero assets**. This is exactly how v3.1.0–v3.3.0 shipped with no binaries while `go install …@latest` kept working.

**Fix (in `.goreleaser.yml`):** one `archives` entry per build filtered by **`builds:`** (NOT `ids:`), plus `version: 2` and `snapshot.version_template` (not the deprecated `name_template`). Any new build target must get its own archive or match an existing one's platform set.

**Critical version trap:** the CI goreleaser is NOT the latest — it is a `GoReleaser@dev` **bundled inside go-semantic-release/action@v1** (v2.31.0 as of this writing), and it is OLDER than `goreleaser/v2@latest`. That bundled version rejects `archives.ids` ("field ids not found in type config.Archive") — it only knows the classic `archives.builds`. `goreleaser/v2@latest` accepts `builds` too but reports it DEPRECATED, so `goreleaser check` "fails" locally with "configuration is valid, but uses deprecated properties" — that exit code is expected and fine; `builds` is the field that works on both. Do NOT "modernize" it to `ids`.

**Why / how to verify:** run a full `go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean --skip=publish` before merging (look for "release succeeded" and the split archives) — but remember local latest is newer than CI, so `check` deprecation errors are not blockers. CI won't catch archive errors until the release job on main, and by then the tag is already burned (v3.1.0–v3.3.1 all shipped empty this way).

**How to apply:** after adding/altering a goreleaser build, run the snapshot dry-run; confirm every produced `.tar.gz` is named per its build and no platform is missing.
