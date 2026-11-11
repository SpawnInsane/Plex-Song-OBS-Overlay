PREFIX ?= $(HOME)/.local
DESTDIR ?=
GO ?= go
BINARY = build/plex-song-obs-overlay

.PHONY: build install uninstall test

build:
	mkdir -p build
	CGO_ENABLED=0 $(GO) build -mod=readonly -trimpath -buildvcs=false -o $(BINARY) .

test:
	$(GO) test ./...

# Run `make build` first; installation never builds or downloads code as root.
install:
	install -Dm755 $(BINARY) "$(DESTDIR)$(PREFIX)/bin/plex-song-obs-overlay"
	install -Dm644 packaging/linux/plex-song-obs-overlay.desktop "$(DESTDIR)$(PREFIX)/share/applications/plex-song-obs-overlay.desktop"
	install -Dm644 assets/plex-song-obs-overlay.png "$(DESTDIR)$(PREFIX)/share/pixmaps/plex-song-obs-overlay.png"
	install -Dm644 LICENSE "$(DESTDIR)$(PREFIX)/share/licenses/plex-song-obs-overlay/LICENSE"
	install -Dm644 README.md "$(DESTDIR)$(PREFIX)/share/doc/plex-song-obs-overlay/README.md"

uninstall:
	rm -f "$(DESTDIR)$(PREFIX)/bin/plex-song-obs-overlay"
	rm -f "$(DESTDIR)$(PREFIX)/share/applications/plex-song-obs-overlay.desktop"
	rm -f "$(DESTDIR)$(PREFIX)/share/pixmaps/plex-song-obs-overlay.png"
	rm -f "$(DESTDIR)$(PREFIX)/share/licenses/plex-song-obs-overlay/LICENSE"
	rm -f "$(DESTDIR)$(PREFIX)/share/doc/plex-song-obs-overlay/README.md"
