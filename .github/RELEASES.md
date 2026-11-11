# Release automation

Development work branches from `develop` and returns to `develop` through pull requests. Merging into `develop` creates release candidates only while preparing a minor or explicitly requested major release; patch-only cycles do not publish release candidates. A `develop` to `main` pull request prepares the stable version, and merging it creates the stable GitHub release.

Commit subjects control semantic versioning:

- `fix:` produces a patch release.
- `feat:` produces a minor release.
- `feat!:` or a `BREAKING CHANGE:` footer is treated as minor unless `.github/release-major-version` explicitly requests the next major.
- `chore:` and other maintenance-only commit or pull-request titles do not produce a release or change the version.

Each release contains the console-free Windows executable, Linux `.deb` packages and portable installation trees for amd64 and arm64, and a source archive. Download artifacts have individual SHA-256 checksum files. Debian release candidates use `~rc.N` so a stable version sorts after its candidates. Curated user-facing notes accumulate in `body.md`; GitHub-generated commit notes are appended automatically.

## Required repository setup

1. Run `powershell -ExecutionPolicy Bypass -File scripts/setup-git-hooks.ps1` once after each new clone to enable automatic messages for blank commits.
2. Add a fine-grained personal access token as the Actions secret `RELEASE_SYNC_TOKEN`. Limit it to this repository with **Contents: Read and write** permission.
3. Create a dedicated Ed25519 SSH key for release automation. Add its public key to the `SpawnInsane` GitHub account as a **Signing key**, then store the complete private key as the Actions secret `RELEASE_SIGNING_PRIVATE_KEY`. Do not reuse an authentication key.
4. Protect `main` and `develop`, require signed commits and pull requests, and require the **Build and test** and **Action pinning** checks.
5. Enable Renovate for the repository if it is not already installed.

The `RELEASE_SYNC_TOKEN` is used only to push signed version-metadata commits and guarded release-note resets. The dedicated signing key proves who created those commits but cannot push by itself. Release creation uses the workflow-scoped `GITHUB_TOKEN`.

To intentionally start a major release, place the positive major number alone in `.github/release-major-version`, for example `1`. Remove the file after that major has been released.

## Linux packages

`python3 scripts/build-linux-packages.py` builds both supported Linux architectures using Go with CGO disabled and packages the executable with a desktop launcher, icon, documentation, and license. It requires Python 3.9+, Go 1.26+, and `dpkg-deb`; use `--arch amd64` or `--arch arm64` for one target, or `--portable-only` to omit Debian packages. Builds run without root privileges. CI tests package layout, Debian version ordering, and signed APT metadata, then builds both architectures. Packages have no maintainer scripts, service, or automatic application startup.

The included AUR package is `plex-song-obs-overlay-git`, building upstream `main`. After Linux support has reached `main`, review and test it with `makepkg -si` as a regular user on Arch. Regenerate metadata using `makepkg --printsrcinfo > .SRCINFO` in `packaging/aur`, then submit `PKGBUILD` and `.SRCINFO` to the package's AUR Git repository using your AUR account. Do not upload built binaries or the full application repository to AUR. Submission is a separate maintainer action; this repository does not store AUR push credentials or publish to AUR automatically.

## Signed APT repository setup

The optional `Publish APT repository` workflow deploys a dedicated package repository to GitHub Pages. It replaces the Pages site with package metadata and packages, so use it only if this project's Pages site is reserved for APT. The documented default URL is `https://spawninsane.github.io/Plex-Song-OBS-Overlay/`; adjust user instructions if using a custom domain or a fork. This is a third-party repository, not an upload to Debian's or Ubuntu's official repositories.

1. In repository **Settings → Pages**, choose **GitHub Actions** as the deployment source. Configure the `github-pages` environment to allow deployments only from `main`.
2. On a trusted machine, create a dedicated GPG signing key for APT. Keep a protected offline backup and its revocation certificate. This key must be separate from personal Git signing and `RELEASE_SIGNING_PRIVATE_KEY`.
3. Store the ASCII-armored private key as the Actions secret `APT_SIGNING_PRIVATE_KEY`. The workflow uses unattended signing, so this automation key must be exported without a passphrase; secure access through GitHub Actions secrets and environment/branch protections. Never put it in source control or logs.
4. Set repository variable `APT_SIGNING_KEY_FINGERPRINT` to its full fingerprint (40 hexadecimal characters for a v4 key). Publish that public fingerprint in the release announcement so users can verify the key before adding the repository.
5. After a stable release containing both `.deb` files is available, set repository variable `PUBLISH_APT_REPOSITORY=true`. Run **Publish APT repository** manually on `main` for the first deployment. Subsequent stable releases invoke it automatically; release candidates never enter the stable APT repository.
6. Test the README's scoped `signed-by` setup and `apt update`/`apt install` commands on Debian or Ubuntu before announcing availability. Renew or rotate the signing key before it expires; users must explicitly replace their scoped keyring when rotating to a new key.

The workflow downloads the latest stable release's `.deb` files using a read-only token, creates per-architecture indices and by-hash copies, signs `Release` and `InRelease`, and exports only the public key. The signing keyring is temporary and removed after signing. Only the separate deployment job can deploy to Pages. The current repository snapshot retains the latest stable package version; use GitHub release downloads for older versions and release candidates. No public packages or hosting are created until a maintainer enables publishing.
