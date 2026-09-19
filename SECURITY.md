# Security Policy

## Supported versions

Security fixes are provided for the latest tagged release and the current `main` branch. Development branches receive fixes on a best-effort basis.

## Reporting a vulnerability

Do not disclose suspected vulnerabilities in public issues, discussions, or pull requests. Report them privately through [GitHub Security Advisories](https://github.com/SpawnInsane/Plex-Song-OBS-Overlay/security/advisories/new) and include the affected version, reproduction steps, impact, and any suggested remediation. Never include a real Plex token or other credential.

## Security boundaries

Plex Song OBS Overlay is a local Windows application. Its web server is bound to `127.0.0.1:7070` and is not intended to be exposed to a LAN or the public internet. It connects from the local process to the operator-configured Plex server.

The desktop control window embeds the Windows WebView2 Runtime and loads the same loopback-hosted interface used by the OBS overlay. WebView2 may store browser data in the current user's local cache, but the application does not use that storage for Plex credentials; saved credentials remain in the application configuration file.

The Plex token is sensitive and is stored in the current user's configuration directory. The token must never be returned by the settings API, included in the overlay URL, logged, or committed to this repository. Anyone with access to the user's Windows account and configuration files may be able to read it.

The loopback server accepts only its canonical `127.0.0.1:7070` authority. State-changing browser requests must use the canonical same origin and the unpredictable per-process control token. Control pages must deny framing so that another site cannot disguise settings or shutdown actions.

Plex responses and artwork are untrusted remote input and must remain size-bounded. The artwork proxy and all other token-bearing Plex requests must remain on the configured Plex scheme, hostname, and effective port across redirects. Non-loopback HTTP exposes the Plex token on the network and is supported only through an explicit, visibly warned operator opt-in; HTTPS is the default security boundary.

The update check contacts the public GitHub Releases API for this repository and downloads release assets from GitHub. Update traffic must remain on HTTPS and on GitHub-owned hosts across redirects, and downloaded responses must remain size-bounded. A downloaded executable must never be installed unless its SHA-256 digest matches the checksum published in the same release; a mismatch must discard the download and leave the running application untouched. Release binaries are not Authenticode-signed, so the published checksum provides integrity against a corrupted or altered download but does not by itself prove publisher authenticity. Installing an update is always an explicit operator action and must never happen automatically. Update download addresses are internal details and must not be returned to the control window.

GitHub release workflows may write tags, releases, release artifacts, version metadata, and release-note resets. External actions used by privileged workflows must remain pinned to reviewed commit digests in every supported YAML representation; structural validation must reject mutable, aliased, or unsupported action references. Write credentials must be limited to the jobs that require them. Automated commits are signed with a dedicated SSH key stored as the `RELEASE_SIGNING_PRIVATE_KEY` Actions secret. That key must be registered only as a GitHub signing key, must not be reused for authentication, and must be rotated immediately if workflow access or the secret is compromised.
