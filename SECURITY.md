# Security Policy

## Supported versions

Security fixes are provided for the latest tagged release and the current `main` branch. Development branches receive fixes on a best-effort basis.

## Reporting a vulnerability

Do not disclose suspected vulnerabilities in public issues, discussions, or pull requests. Report them privately through [GitHub Security Advisories](https://github.com/SpawnInsane/Plex-Song-OBS-Overlay/security/advisories/new) and include the affected version, reproduction steps, impact, and any suggested remediation. Never include a real Plex token or other credential.

## Security boundaries

Plex Song OBS Overlay is a local Windows application. Its web server is bound to `127.0.0.1:7070` and is not intended to be exposed to a LAN or the public internet. It connects from the local process to the operator-configured Plex server.

The Plex token is sensitive and is stored in the current user's configuration directory. The token must never be returned by the settings API, included in the overlay URL, logged, or committed to this repository. Anyone with access to the user's Windows account and configuration files may be able to read it.

State-changing browser requests must remain same-origin. Plex responses and artwork are untrusted remote input and must remain size-bounded. The artwork proxy must only contact the configured Plex origin and accept image responses.

GitHub release workflows may write tags, releases, release artifacts, version metadata, and release-note resets. External actions used by privileged workflows must remain pinned to reviewed commit digests, and write credentials must be limited to the jobs that require them.
