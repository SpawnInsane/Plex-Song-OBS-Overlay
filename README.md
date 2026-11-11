<p align="center">
  <img src="assets/plex-song-obs-overlay.png" alt="Plex Song OBS Overlay icon" width="180">
</p>

# Plex Song OBS Overlay

Plex Song OBS Overlay is a small Windows and Linux application that displays the song currently playing for a Plex user as a transparent Twitch/OBS overlay.

## Install on Linux

Linux requires a graphical desktop, a default web browser, `xdg-utils`, and CA certificates. Packages support x86-64 (`amd64` on Debian, `x86_64` on Arch) and ARM64 (`arm64` / `aarch64`). Run the application as your normal desktop user.

The Linux downloads and package-manager commands below become available after the first release containing Linux support. The AUR listing and APT repository also need to be published before installing by package name. Until then, build from a checkout containing this change using the source instructions below.

### Debian, Ubuntu, Linux Mint, and other APT distributions

Download the matching `.deb` from [GitHub Releases](https://github.com/SpawnInsane/Plex-Song-OBS-Overlay/releases/latest). In the directory containing the file, install it with:

```sh
sudo apt update
sudo apt install ./plex-song-obs-overlay_*_amd64.deb
# On ARM64, use the *_arm64.deb file instead.
```

To receive updates through `apt install`, `apt-get install`, and your normal system updater, add the project's signed repository once it is published:

```sh
sudo apt install ca-certificates curl gnupg
curl -fL -o plex-song-obs-overlay-archive-keyring.asc \
  https://spawninsane.github.io/Plex-Song-OBS-Overlay/plex-song-obs-overlay-archive-keyring.asc
gpg --show-keys --fingerprint plex-song-obs-overlay-archive-keyring.asc
```

Compare the displayed fingerprint with the APT key fingerprint published by the maintainer before trusting the key. Then:

```sh
sudo install -d -m755 /etc/apt/keyrings
sudo install -m644 plex-song-obs-overlay-archive-keyring.asc /etc/apt/keyrings/
echo 'deb [arch=amd64,arm64 signed-by=/etc/apt/keyrings/plex-song-obs-overlay-archive-keyring.asc] https://spawninsane.github.io/Plex-Song-OBS-Overlay stable main' \
  | sudo tee /etc/apt/sources.list.d/plex-song-obs-overlay.list
sudo apt update
sudo apt install plex-song-obs-overlay
# Alternatively: sudo apt-get install plex-song-obs-overlay
```

The repository serves stable releases. Pre-release `.deb` packages can be installed manually from GitHub Releases. Stop the application before upgrading, then launch it again. Package upgrades and removal leave your personal settings intact.

### Arch Linux, CachyOS, and Manjaro (AUR)

Once the AUR package has been submitted, use your usual AUR helper:

```sh
yay -S plex-song-obs-overlay-git
# Or: paru -S plex-song-obs-overlay-git
```

Or review and build the AUR recipe yourself as a regular user:

```sh
git clone https://aur.archlinux.org/plex-song-obs-overlay-git.git
cd plex-song-obs-overlay-git
less PKGBUILD
makepkg -si
```

This package builds the latest source on the project's `main` branch. To build the included recipe directly from a GitHub checkout, run `cd packaging/aur` followed by `makepkg -si` after the Linux changes reach `main`.

### Launch and stop

Select **Plex Song OBS Overlay** in your applications menu, or run `plex-song-obs-overlay` in a terminal. The control page opens in your default browser. Use it to save Plex settings and obtain the OBS overlay URL.

Closing the browser tab leaves the Linux application running. Select **Stop application** on the control page to exit, or press `Ctrl+C` if launched from a terminal. Launching the application again opens the controls for the existing instance. The application does not install a system service or start automatically at login.

Settings are stored in `${XDG_CONFIG_HOME:-$HOME/.config}/Plex Song OBS Overlay/config.json`. They belong to your desktop user and contain your Plex token. Keep this file private.

## Quick start

1. On Windows, double-click `PlexSongOBSOverlay.exe`. On Linux, launch **Plex Song OBS Overlay** from your applications menu or terminal.
2. The control window opens on Windows; Linux opens the control page in your default browser.
3. Enter your Plex server URL, Plex token, and optional Plex username.
4. Select **Save settings**, then **Test connection**.
5. Copy the overlay URL shown in the control window into an OBS Browser Source.

Your settings are saved automatically in your user configuration folder and remain available after the application closes. The control page displays the exact file location. The Plex token is never included in the overlay URL or returned to the control page after it is saved.

The application must be running while you stream. On Windows, keep the primary control window open while the overlay is in use; closing it exits the program. On Linux, the browser tab can be closed while streaming. Selecting **Stop application** exits the program on either platform.

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

The card stays transparent and hidden when the selected user is not playing music. It displays album artwork, title, artist, album, smoothly animated progress, current and total track times, and paused status. Song changes and playback seeks reset the progress position automatically.

## Security notes

Plex Song OBS Overlay listens only on `127.0.0.1`, so other computers cannot open its settings page. The Plex token is saved locally in the current user's configuration file. Do not share that file or commit it to source control.

## Build from source

Go 1.26 or newer is required.

### Download from GitHub and build on Linux

Install `make`, a Go 1.26+ toolchain, `xdg-utils`, and CA certificates. Debian/Ubuntu users can install the supporting tools with `sudo apt install build-essential ca-certificates curl xdg-utils`; if the distribution's Go package is older than 1.26, install a current toolchain using the [official Go installation instructions](https://go.dev/doc/install). Arch users can run `sudo pacman -S --needed base-devel go git ca-certificates xdg-utils`.

After a Linux-enabled release is published, download its source and checksum:

```sh
curl -fLO https://github.com/SpawnInsane/Plex-Song-OBS-Overlay/releases/latest/download/plex-song-obs-overlay-source.tar.gz
curl -fLO https://github.com/SpawnInsane/Plex-Song-OBS-Overlay/releases/latest/download/plex-song-obs-overlay-source.tar.gz.sha256
sha256sum -c plex-song-obs-overlay-source.tar.gz.sha256
tar -xzf plex-song-obs-overlay-source.tar.gz
cd plex-song-obs-overlay-source
```

Alternatively, download the source through Git:

```sh
git clone https://github.com/SpawnInsane/Plex-Song-OBS-Overlay.git
cd Plex-Song-OBS-Overlay
```

From either source directory, build, test, and install for your user:

```sh
go version
make build
make test
make install
export PATH="$HOME/.local/bin:$PATH"
plex-song-obs-overlay
```

The executable is installed to `~/.local/bin` with a launcher under `~/.local/share/applications`. If your desktop session does not yet include `~/.local/bin` in `PATH`, add it to your shell profile and sign out and back in before using the launcher. To install system-wide instead, build as your normal user, then run `sudo make install PREFIX=/usr/local`.

To remove a source installation, run `make uninstall` from the source directory (or `sudo make uninstall PREFIX=/usr/local` for a system-wide install). Your Plex settings are kept. Rebuild and reinstall from a newer download to update a source installation.

To create your own `.deb` installers and portable archives, also install Python 3.9+ and `dpkg-deb` (`sudo apt install python3 dpkg` on Debian/Ubuntu), then run:

```sh
python3 scripts/build-linux-packages.py --arch amd64
# Use --arch arm64 for ARM64, or omit --arch to build both.
# Add --portable-only to build without dpkg-deb.
```

Files appear in `dist/`. Install a local `.deb` with `sudo apt install ./dist/plex-song-obs-overlay_*_amd64.deb`. The portable archive contains an installation tree under `usr/`; it includes the executable, desktop launcher, icon, documentation, and license.

### Windows

The Windows application uses the Microsoft Edge WebView2 Runtime for its desktop control window. WebView2 is included with Windows 11 and most current Windows 10 installations. If the application reports that WebView2 is unavailable, install Microsoft's [Evergreen WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/consumer/) and start the application again.

For the normal Windows application without a console window:

```powershell
go build -ldflags "-H=windowsgui" -o PlexSongOBSOverlay.exe .
```

For a development build that keeps diagnostic output visible (non-Windows development opens the control page in the default browser):

```powershell
go run .
```
