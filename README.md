# Plex Song OBS Overlay

Plex Song OBS Overlay is a small Windows application that displays the song currently playing for a Plex user as a transparent Twitch/OBS overlay.

## Quick start

1. Double-click `PlexSongOBSOverlay.exe`.
2. The setup page opens automatically in your web browser.
3. Enter your Plex server URL, Plex token, and optional Plex username.
4. Select **Save settings**, then **Test connection**.
5. Copy the overlay URL shown on the setup page into an OBS Browser Source.

Your settings are saved automatically in your Windows user configuration folder and remain available after the application closes. The setup page displays the exact file location. The Plex token is never included in the overlay URL or returned to the browser after it is saved.

The application must be running while you stream. Use **Stop application** at the bottom of the setup page when you are finished. Opening the executable again while it is already running simply reopens the setup page.

## Plex settings

- **Plex server URL:** Usually `http://PLEX-COMPUTER-IP:32400`, such as `http://192.168.1.100:32400`.
- **Plex token:** Follow Plex's guide to [find your authentication token](https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/). Treat this token like a password.
- **Plex username:** The user whose playback should appear. Leave this blank to show the first active music session from any user.
- **Refresh interval:** How frequently the overlay asks Plex for an update. Three seconds is recommended.

## Add the overlay to OBS

1. In OBS, select **Sources**, then **Add** and **Browser**.
2. Enter `http://127.0.0.1:7070/web/overlay.html` as the URL.
3. Set the width to `620` and height to `160`.
4. Optionally enable **Refresh browser when scene becomes active**.

The card stays transparent and hidden when the selected user is not playing music. It displays album artwork, title, artist, album, progress, and paused status.

## Security notes

Plex Song OBS Overlay listens only on `127.0.0.1`, so other computers cannot open its settings page. The Plex token is saved locally in the current Windows user's configuration file. Do not share that file or commit it to source control.

## Build from source

Go 1.24 or newer is required.

For the normal Windows application without a console window:

```powershell
go build -ldflags "-H=windowsgui" -o PlexSongOBSOverlay.exe .
```

For a development build that keeps diagnostic output visible:

```powershell
go run .
```

No third-party Go packages are required.
