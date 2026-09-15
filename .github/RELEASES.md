# Release automation

Development work branches from `develop` and returns to `develop` through pull requests. Merging a pull request into `develop` creates a release candidate. A `develop` to `main` pull request prepares the stable version, and merging it creates the stable GitHub release.

Commit subjects control semantic versioning:

- `fix:` produces a patch release.
- `feat:` produces a minor release.
- `feat!:` or a `BREAKING CHANGE:` footer is treated as minor unless `.github/release-major-version` explicitly requests the next major.

Each release contains the console-free Windows executable and a SHA-256 checksum. Curated user-facing notes accumulate in `body.md`; GitHub-generated commit notes are appended automatically.

## Required repository setup

1. Run `powershell -ExecutionPolicy Bypass -File scripts/setup-git-hooks.ps1` once after each new clone to enable automatic messages for blank commits.
2. Add a fine-grained personal access token as the Actions secret `RELEASE_SYNC_TOKEN`. Limit it to this repository with **Contents: Read and write** permission.
3. Protect `main` and `develop`, require pull requests, and require the **Build and test** and **Action pinning** checks.
4. Enable Renovate for the repository if it is not already installed.

The `RELEASE_SYNC_TOKEN` is used only for version-metadata commits and guarded release-note resets. Release creation uses the workflow-scoped `GITHUB_TOKEN`.

To intentionally start a major release, place the positive major number alone in `.github/release-major-version`, for example `1`. Remove the file after that major has been released.
