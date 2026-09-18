<p align="center">
  <img src="assets/plex-song-obs-overlay.png" alt="Plex Song OBS Overlay icon" width="180">
</p>

# Plex Song OBS Overlay

Plex Song OBS Overlay is a small Windows application that displays the song currently playing for a Plex user as a transparent Twitch/OBS overlay.

## Quick start

1. Double-click `PlexSongOBSOverlay.exe`.
2. The Plex Song OBS Overlay control window opens on your desktop.
3. Enter your Plex server URL, Plex token, and optional Plex username.
4. Select **Save settings**, then **Test connection**.
5. Copy the overlay URL shown in the control window into an OBS Browser Source.

Your settings are saved automatically in your Windows user configuration folder and remain available after the application closes. The control window displays the exact file location. The Plex token is never included in the overlay URL or returned to the control window after it is saved.

The application must be running while you stream. Keep the control window open while the overlay is in use. Closing the primary window or selecting **Stop application** exits the program. Opening the executable again while it is already running opens another control window for the running program; any additional windows close automatically when the primary application stops.

## Plex settings

- **Plex server URL:** Prefer an HTTPS address for Plex. HTTP is accepted automatically only for Plex on this computer. A trusted LAN server such as `http://192.168.1.100:32400` requires selecting **Allow insecure HTTP** after acknowledging that the token and playback traffic will not be encrypted.
- **Plex token:** Follow Plex's guide to [find your authentication token](https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/). Treat this token like a password.
- **Plex username:** The user whose playback should appear. Leave this blank to show the first active music session from any user.
- **Refresh interval:** How frequently the overlay asks Plex for an update. Three seconds is recommended.

## Add the overlay to OBS

1. In OBS, select **Sources**, then **Add** and **Browser**.
2. Enter `http://127.0.0.1:7070/web/overlay.html` as the URL.
3. Set the width to `620` and height to `160`.
4. Optionally enable **Refresh browser when scene becomes active**.

The card stays transparent and hidden when the selected user is not playing music. It displays album artwork, title, artist, album, smoothly animated progress, current and total track times, and paused status. Song changes and playback seeks reset the progress position automatically.

## Security notes

Plex Song OBS Overlay listens only on `127.0.0.1`, rejects other host names, and authorizes control-window changes with a temporary token created each time the application starts. The Plex token is saved locally in the current Windows user's configuration file. Do not share that file or commit it to source control.

Use HTTPS for Plex whenever possible. Enabling insecure HTTP for a remote or LAN server allows devices on that network path to observe or alter the Plex token and playback traffic.

## Build from source

Go 1.26 or newer is required.

The Windows application uses the Microsoft Edge WebView2 Runtime for its desktop control window. WebView2 is included with Windows 11 and most current Windows 10 installations. If the application reports that WebView2 is unavailable, install Microsoft's [Evergreen WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/consumer/) and start the application again.

For the normal Windows application without a console window:

```powershell
go build -ldflags "-H=windowsgui" -o PlexSongOBSOverlay.exe .
```

For a development build that keeps diagnostic output visible (non-Windows development opens the control page in the default browser):

```powershell
go run .
```
