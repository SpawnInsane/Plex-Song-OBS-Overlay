# Repository instructions

- Whenever you make changes to this repository, append concise, user-facing release notes to `body.md`.
- Keep `README.md` focused on the Plex Song Grabber program: installation, configuration, user-visible behavior, and operator guidance. Do not document repository, CI, release, or contributor automation there.
- Update `SECURITY.md` whenever changes affect security boundaries, local credential storage, network exposure, or workflow security.
- Create development branches from `develop` and merge completed work back into `develop`. Accumulate changes there, then merge `develop` into `main` only when the application is ready for a stable release.
- Use Conventional Commit subjects. Use `fix:` for patch-impact changes and `feat:` for minor-impact changes. Use `feat!:` with a `BREAKING CHANGE:` footer only for genuine breaking changes.
- When asked to generate a commit message, inspect the current staged diff and recent history, then return only the commit message unless the user explicitly asks to commit.
- Keep `.githooks/prepare-commit-msg` enabled through `core.hooksPath=.githooks`. It supplies a Conventional Commit subject only when the user leaves the commit message blank and must not replace explicit, merge, squash, or amend messages.
- Never let automated versioning select a new major version. Only when the user explicitly requests a major release, create or update `.github/release-major-version` with the requested positive major number; otherwise leave that file absent.
- Accumulate `body.md` entries throughout a development cycle. Do not overwrite existing entries. Reset it only after a `develop`-to-`main` release when beginning the next cycle.
- Before opening a pull request, compare the base and head branches and check for duplicate open pull requests. Read back the created pull request state, and never merge without explicit user direction.
