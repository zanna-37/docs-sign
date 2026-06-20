# CLAUDE.md

Project notes for docs-sign.

## Versioning

The app version is derived from **git tags** and stamped into the binary at link time.
There is one source of truth — the git tag — which flows everywhere:

```
git tag (vX.Y.Z) → git describe → -ldflags -X → internal/version → GET /api/version → web UI footer
```

- `internal/version/version.go` holds `Version` and `Commit`, defaulting to `dev`/`none`
  for unstamped builds (e.g. `go run`).
- The value comes from `git describe --tags --always --dirty`:
  - on a tagged commit → `v1.2.3`
  - commits past a tag → `v1.2.3-4-gabc1234`
  - a repo with no tags → the short commit SHA
- `web/package.json`'s `version` field is **not** used for display; ignore it.
- The version is visible in the app footer, on the login screen, and at `GET /api/version`.

## Cutting a release

A release is just an annotated git tag of the form `vMAJOR.MINOR.PATCH`. Pushing it
triggers `.github/workflows/docker.yml`, which builds the multi-arch image, stamps it with
the tag version, and pushes it to Docker Hub.

1. Make sure `main` is green and up to date:
   ```sh
   git checkout main && git pull
   ```
2. Choose the next [semver](https://semver.org/) version and create an annotated tag:
   ```sh
   git tag -a v1.2.3 -m "v1.2.3"
   ```
3. Push the tag:
   ```sh
   git push origin v1.2.3
   ```
4. Watch the **Build and push image** workflow under the repo's Actions tab. On success it
   publishes these Docker Hub tags (see `docker/metadata-action` in the workflow):
   - `1.2.3` (full version)
   - `1.2` (major.minor — moves forward with each patch)
   - `latest` is published from `main` pushes, **not** from tags.
5. Verify the published image reports the right version:
   ```sh
   docker run --rm <namespace>/docs-sign:1.2.3 docs-sign --help   # sanity check it starts
   ```
   Or, once deployed, check `GET /api/version` / the app footer.

To undo a mistagged release before it ships, delete the tag locally and remotely
(`git tag -d v1.2.3 && git push origin :refs/tags/v1.2.3`) and re-tag. Never move an
already-published tag.

## Building locally

- `make build` — builds the embedded frontend + the single binary, version stamped from
  `git describe` automatically.
- `make build VERSION=v1.2.3` — override the stamped version (e.g. for a local test build).
- Docker builds receive the version via the `VERSION` / `COMMIT` build args (the `.git`
  dir is not in the build context, so `git describe` can't run inside the image); CI passes
  them automatically.
