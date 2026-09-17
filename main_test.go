package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCurrentTrackSelectsConfiguredUser(t *testing.T) {
	const sessions = `<MediaContainer size="2">
<Track type="track" title="Wrong Song" grandparentTitle="Other Artist"><User title="Other"/><Player state="playing"/></Track>
<Track type="track" title="Right Song" grandparentTitle="Artist" parentTitle="Album" thumb="/library/metadata/1/thumb/2" duration="240000" viewOffset="12000"><User title="Sara"/><Player state="paused"/></Track>
</MediaContainer>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Plex-Token"); got != "secret" {
			t.Errorf("token header = %q", got)
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(sessions))
	}))
	defer server.Close()

	base, _ := url.Parse(server.URL)
	client := &plexClient{base: base, token: "secret", client: server.Client()}
	got, found, err := client.currentTrack(context.Background(), "sara")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected a track")
	}
	if got.Title != "Right Song" || got.Player.State != "paused" || got.Duration != 240000 {
		t.Fatalf("unexpected track: %+v", got)
	}
}

func TestCurrentTrackReturnsNotFoundWithoutMusic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<MediaContainer><Video type="movie"><User title="Sara"/><Player state="playing"/></Video></MediaContainer>`))
	}))
	defer server.Close()
	base, _ := url.Parse(server.URL)
	client := &plexClient{base: base, client: server.Client()}
	_, found, err := client.currentTrack(context.Background(), "Sara")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("did not expect a music track")
	}
}

func TestSettingsPersistAndTokenIsNotReturned(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	store, err := newSettingsStore(path)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(routes(store, func() {}))
	defer server.Close()

	payload := []byte(`{"plexUrl":"http://plex.local:32400","plexToken":"very-secret","plexUser":"Sara","pollIntervalSeconds":5}`)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/settings", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("save status = %d", response.StatusCode)
	}

	reloaded, err := newSettingsStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.snapshot(); got.PlexToken != "very-secret" || got.PlexUser != "Sara" || got.PollIntervalSeconds != 5 {
		t.Fatalf("unexpected persisted settings: %+v", got)
	}

	response, err = server.Client().Get(server.URL + "/api/settings")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var public map[string]any
	if err := json.NewDecoder(response.Body).Decode(&public); err != nil {
		t.Fatal(err)
	}
	if _, exposed := public["plexToken"]; exposed {
		t.Fatal("settings response exposed the Plex token")
	}
	if configured, _ := public["tokenConfigured"].(bool); !configured {
		t.Fatal("expected tokenConfigured to be true")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("settings file was not created: %v", err)
	}
}

func TestSettingsRejectCrossOriginWrite(t *testing.T) {
	store, err := newSettingsStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(routes(store, func() {}))
	defer server.Close()
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/settings", bytes.NewReader([]byte(`{}`)))
	request.Header.Set("Origin", "https://example.com")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d", response.StatusCode)
	}
}

func TestMigrateLegacySettings(t *testing.T) {
	directory := t.TempDir()
	legacyPath := filepath.Join(directory, legacyProductName, "config.json")
	currentPath := filepath.Join(directory, productName, "config.json")
	legacy, err := newSettingsStore(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	want := savedSettings{PlexURL: "http://plex.local:32400", PlexToken: "secret", PlexUser: "Sara", PollIntervalSeconds: 3}
	if err := legacy.save(want); err != nil {
		t.Fatal(err)
	}
	current, err := newSettingsStore(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrateLegacySettings(current, legacyPath); err != nil {
		t.Fatal(err)
	}
	if got := current.snapshot(); got != want {
		t.Fatalf("migrated settings = %+v, want %+v", got, want)
	}
}

func TestMonitorServerShutdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))
	stopped, stopMonitoring := monitorServerShutdown(server.URL, 10*time.Millisecond)
	defer stopMonitoring()

	select {
	case <-stopped:
		t.Fatal("monitor reported a healthy server as stopped")
	case <-time.After(30 * time.Millisecond):
	}

	server.Close()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("monitor did not report the stopped server")
	}
}
