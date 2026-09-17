# Release notes

- Replaced the browser-based setup experience with a dedicated desktop control window that closes the application when dismissed.
- Limited release-candidate builds to minor and explicitly requested major releases, so patch fixes proceed directly to the next stable release.
- Ensured additional control windows close automatically when the primary application stops.
- Smoothed the playback progress bar and made it reset reliably after song changes, skips, and rewinds.
- Added current and total track times beside the playback progress bar.
- Added a new project logo and matching Windows executable, application-window, and taskbar icon.
- Fixed release automation so its version and release-note commits satisfy signed-commit protection.
- Corrected the source-build guide to require the Go version declared by the project.
- Prevented the playback timer and progress bar from jumping backward when Plex reports a slightly stale position.
