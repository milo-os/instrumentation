# Releasing

Each language is versioned on its own and tagged with its directory as a
prefix. Never create an unprefixed `vX.Y.Z` tag: v0.1.0 to v0.3.0 were tagged
that way before the Go module moved into `go/`, and that namespace is closed.

Release only what is merged to `main` with CI passing. Choose the version by
semver; while the version is 0.x, bump the minor version for any new feature
or breaking change.

## Go: `go/vX.Y.Z`

1. Find the merge commit to release:

   ```sh
   git fetch origin --tags
   git log --oneline -3 origin/main
   ```

1. Tag it and push the tag:

   ```sh
   git tag -a go/vX.Y.Z <commit> -m go/vX.Y.Z
   git push origin go/vX.Y.Z
   ```

1. Create the GitHub release, with notes listing what changed for users:

   ```sh
   gh release create go/vX.Y.Z -R milo-os/instrumentation --verify-tag \
     --title go/vX.Y.Z --notes "<what changed>"
   ```

1. Verify that the version resolves through the module proxy and directly.
   Go names the version without the `go/` prefix:

   ```sh
   GOPROXY=https://proxy.golang.org go list -m go.miloapis.com/instrumentation@vX.Y.Z
   GOPROXY=direct go list -m go.miloapis.com/instrumentation@vX.Y.Z
   ```

   `@latest` through the proxy can lag the tag by a few minutes.

## TypeScript: `ts/vX.Y.Z`

npm's trusted publisher for this package accepts only staged publishing. CI
stages each version, and a maintainer must approve it with two-factor
authentication before it reaches npm's `latest` tag. CI credentials can't
approve, so an agent can't either: a person with publish access on npm must.

1. Start the release with the bump that matches the change, `patch`,
   `minor`, or `major`:

   ```sh
   gh workflow run publish-ts.yaml -R milo-os/instrumentation --ref main -f bump-type=patch
   ```

   The workflow opens a pull request titled
   `chore(ts): release @miloapis/instrumentation vX.Y.Z` that bumps
   `ts/package.json`.

1. Start CI on that pull request. GitHub doesn't run workflows for a pull
   request opened by a workflow, and `main` requires the `ci` check, so the
   pull request stays blocked until a person closes and reopens it:

   ```sh
   gh pr close <number> -R milo-os/instrumentation
   gh pr reopen <number> -R milo-os/instrumentation
   ```

1. A person reviews and merges the pull request. The merge stages the version
   on npm, tags `ts/vX.Y.Z`, and creates the GitHub release.

1. Find the stage ID in the merge's workflow run, in the "Stage on npm" step's
   line `staged with id <id>`. Ask a maintainer to approve it, on npmjs.com
   under the package's staged versions, or with:

   ```sh
   npm stage approve <id> --otp=<code>
   ```

1. Verify that the version is `latest`, published by GitHub Actions with
   provenance:

   ```sh
   npm view @miloapis/instrumentation dist-tags.latest _npmUser.name
   ```

   In a new directory, install it and check its signatures:

   ```sh
   npm install @miloapis/instrumentation@X.Y.Z @prometheus-io/client
   npm audit signatures
   ```

### Failures to recognize

- **The pull request shows `ci` as expected, never running.** Step 2 was
  skipped.
- **The staging step fails with 404 or 403.** The trusted publisher on npm no
  longer matches. It must name repository `milo-os/instrumentation` and
  workflow `publish-ts.yaml` exactly. Its fields can't be edited: a maintainer
  deletes it and creates a new one, which must publish within two days to stay
  valid.
- **A push to `ts/` fails to stage a version that's awaiting approval.** The
  version is already staged. This is harmless; approve or reject the staged
  version.
- **A staged version is wrong.** Reject it on npm instead of approving it,
  delete the `ts/vX.Y.Z` tag and its GitHub release, then release a new
  version. A version number can't be reused once it has been staged.
