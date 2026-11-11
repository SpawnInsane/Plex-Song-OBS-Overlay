# Security Policy

## Supported versions

Security fixes are provided for the latest tagged release and the current `main` branch. Development branches receive fixes on a best-effort basis.

## Reporting a vulnerability

Do not disclose suspected vulnerabilities in public issues, discussions, or pull requests. Report them privately through [GitHub Security Advisories](https://github.com/SpawnInsane/Plex-Song-OBS-Overlay/security/advisories/new) and include the affected version, reproduction steps, impact, and any suggested remediation. Never include a real Plex token or other credential.

## Security boundaries

Plex Song OBS Overlay is a local Windows and Linux application. Its web server is bound to `127.0.0.1:7070` and is not intended to be exposed to a LAN or the public internet. It connects from the local process to the operator-configured Plex server.

The desktop control window embeds the Windows WebView2 Runtime and loads the same loopback-hosted interface used by the OBS overlay. WebView2 may store browser data in the current user's local cache, but the application does not use that storage for Plex credentials; saved credentials remain in the application configuration file.

On Linux, the control page opens through `xdg-open` in the user's default browser. Closing the tab does not stop the application. Packages install an executable, desktop launcher, icon, documentation, and license; they must not install a root service, open firewall ports, run the application during installation, or ship user credentials. Run the program as the desktop user. Settings are stored under `$XDG_CONFIG_HOME/Plex Song OBS Overlay`, falling back to `~/.config/Plex Song OBS Overlay`; new configuration directories and files use modes `0700` and `0600` respectively.

The Plex token is sensitive and is stored in the current user's configuration directory. The token must never be returned by the settings API, included in the overlay URL, logged, or committed to this repository. Anyone with access to the user's account and configuration files may be able to read it.

The loopback server accepts only its canonical `127.0.0.1:7070` authority. State-changing browser requests must use the canonical same origin and the unpredictable per-process control token. Control pages must deny framing so that another site cannot disguise settings or shutdown actions.

GitHub release workflows may write tags, releases, release artifacts, version metadata, and release-note resets. External actions used by privileged workflows must remain pinned to reviewed commit digests, and write credentials must be limited to the jobs that require them. Automated commits are signed with a dedicated SSH key stored as the `RELEASE_SIGNING_PRIVATE_KEY` Actions secret. That key must be registered only as a GitHub signing key, must not be reused for authentication, and must be rotated immediately if workflow access or the secret is compromised.

Linux updates are installed through the package manager or by rebuilding from source. The AUR recipe builds the upstream `main` branch as the invoking user; VCS source checksums are skipped by makepkg, so users must review the recipe and trust that upstream. Go dependency verification continues to use `go.sum`.

The optional APT repository is published only from `main` and serves stable amd64 and arm64 packages. Its Release metadata must be signed by a dedicated repository GPG key, stored only in the `APT_SIGNING_PRIVATE_KEY` Actions secret and selected by the full `APT_SIGNING_KEY_FINGERPRINT` repository variable. Never reuse a personal commit-signing key. Import the key into an isolated temporary keyring and delete that keyring after signing. Only the exported public key and signed package metadata may be deployed to Pages. The deploy job alone receives `pages: write` and `id-token: write`; pull requests must not access the APT signing secret. Users must scope trust to this repository with `signed-by`, verify the published fingerprint, and never bypass APT signature verification. Rotate this key and notify users if signing or publishing credentials are compromised.
