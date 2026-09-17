# Release automation

Development work branches from `develop` and returns to `develop` through pull requests. Merging into `develop` creates release candidates only while preparing a minor or explicitly requested major release; patch-only cycles do not publish release candidates. A `develop` to `main` pull request prepares the stable version, and merging it creates the stable GitHub release.

Commit subjects control semantic versioning:

- `fix:` produces a patch release.
- `feat:` produces a minor release.
- `feat!:` or a `BREAKING CHANGE:` footer is treated as minor unless `.github/release-major-version` explicitly requests the next major.
- `chore:` and other maintenance-only commit or pull-request titles do not produce a release or change the version.

Each release contains the console-free Windows executable and a SHA-256 checksum. Curated user-facing notes accumulate in `body.md`; GitHub-generated commit notes are appended automatically.

## Required repository setup

1. Run `powershell -ExecutionPolicy Bypass -File scripts/setup-git-hooks.ps1` once after each new clone to enable automatic messages for blank commits.
2. Add a fine-grained personal access token as the Actions secret `RELEASE_SYNC_TOKEN`. Limit it to this repository with **Contents: Read and write** permission.
3. Create a dedicated Ed25519 SSH key for release automation. Add its public key to the `SpawnInsane` GitHub account as a **Signing key**, then store the complete private key as the Actions secret `RELEASE_SIGNING_PRIVATE_KEY`. Do not reuse an authentication key.
4. Protect `main` and `develop`, require signed commits and pull requests, and require the **Build and test** and **Action pinning** checks.
5. Enable Renovate for the repository if it is not already installed.

The `RELEASE_SYNC_TOKEN` is used only to push signed version-metadata commits and guarded release-note resets. The dedicated signing key proves who created those commits but cannot push by itself. Release creation uses the workflow-scoped `GITHUB_TOKEN`.

To intentionally start a major release, place the positive major number alone in `.github/release-major-version`, for example `1`. Remove the file after that major has been released.
