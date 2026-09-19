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
	"strings"
	"testing"
	"time"
)

func TestCurrentTrackSelectsConfiguredUser(t *testing.T) {
	const sessions = `<MediaContainer size="2">
<Track type="track" title="Wrong Song" grandparentTitle="Other Artist"><User title="Other"/><Player state="playing"/></Track>
<Track type="track" ratingKey="123" key="/library/metadata/123" title="Right Song" grandparentTitle="Artist" parentTitle="Album" thumb="/library/metadata/1/thumb/2" duration="240000" viewOffset="12000"><User title="Sara"/><Player state="paused"/></Track>
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
	if got.RatingKey != "123" || got.Key != "/library/metadata/123" || got.Title != "Right Song" || got.Player.State != "paused" || got.Duration != 240000 {
		t.Fatalf("unexpected track: %+v", got)
	}
}

func TestNowPlayingIncludesStableTrackID(t *testing.T) {
	plex := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<MediaContainer><Track type="track" ratingKey="456" title="Song" duration="180000" viewOffset="15000"><User title="Sara"/><Player state="playing"/></Track></MediaContainer>`))
	}))
	defer plex.Close()

	store := &settingsStore{settings: savedSettings{PlexURL: plex.URL, PlexToken: "secret", PlexUser: "Sara", PollIntervalSeconds: 3}}
	server := httptest.NewServer(routes(store, func() {}, "test-control-token", nil))
	defer server.Close()
	response, err := server.Client().Get(server.URL + "/api/now-playing")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var current nowPlaying
	if err := json.NewDecoder(response.Body).Decode(&current); err != nil {
		t.Fatal(err)
	}
	if current.TrackID != "456" || current.PositionMS != 15000 || !current.Playing {
		t.Fatalf("unexpected now-playing response: %+v", current)
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
	server := httptest.NewServer(routes(store, func() {}, "test-control-token", nil))
	defer server.Close()

	payload := []byte(`{"plexUrl":"http://plex.local:32400","plexToken":"very-secret","plexUser":"Sara","pollIntervalSeconds":5,"allowInsecureHttp":true}`)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/settings", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://"+listenAddr)
	request.Header.Set(controlTokenHeader, "test-control-token")
	request.Host = listenAddr
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
	if got := reloaded.snapshot(); got.PlexToken != "very-secret" || got.PlexUser != "Sara" || got.PollIntervalSeconds != 5 || !got.AllowInsecureHTTP {
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

func TestCanonicalHostRejectsDNSRebindingAuthority(t *testing.T) {
	store := &settingsStore{}
	handler := appHandler(store, func() {}, "test-control-token", nil)
	request := httptest.NewRequest(http.MethodGet, "http://attacker.example:7070/api/control-token", nil)
	request.Host = "attacker.example:7070"
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("rebound host status = %d", response.Code)
	}
}

func TestStateChangesRequireCanonicalOriginAndControlToken(t *testing.T) {
	store := &settingsStore{settings: savedSettings{PlexURL: "https://plex.example", PlexToken: "secret", PollIntervalSeconds: 3}}
	handler := appHandler(store, func() {}, "test-control-token", nil)
	tests := []struct {
		name   string
		origin string
		token  string
		want   int
	}{
		{name: "missing origin", token: "test-control-token", want: http.StatusForbidden},
		{name: "rebound origin", origin: "http://attacker.example:7070", token: "test-control-token", want: http.StatusForbidden},
		{name: "missing token", origin: "http://" + listenAddr, want: http.StatusForbidden},
		{name: "wrong token", origin: "http://" + listenAddr, token: "wrong", want: http.StatusForbidden},
		{name: "authorized", origin: "http://" + listenAddr, token: "test-control-token", want: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://"+listenAddr+"/api/settings", strings.NewReader("{}"))
			request.Host = listenAddr
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.token != "" {
				request.Header.Set(controlTokenHeader, test.token)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
		})
	}
}

func TestSettingsRequireTokenWhenPlexOriginChanges(t *testing.T) {
	store := &settingsStore{settings: savedSettings{PlexURL: "https://plex.example", PlexToken: "saved-secret", PollIntervalSeconds: 3}}
	handler := appHandler(store, func() {}, "test-control-token", nil)
	payload := `{"plexUrl":"https://attacker.example","plexToken":"","pollIntervalSeconds":3}`
	request := httptest.NewRequest(http.MethodPost, "http://"+listenAddr+"/api/settings", strings.NewReader(payload))
	request.Host = listenAddr
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://"+listenAddr)
	request.Header.Set(controlTokenHeader, "test-control-token")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
	if got := store.snapshot(); got.PlexURL != "https://plex.example" || got.PlexToken != "saved-secret" {
		t.Fatalf("settings changed after rejected request: %+v", got)
	}
}

func TestSettingsPreserveTokenForUnchangedPlexOrigin(t *testing.T) {
	store := &settingsStore{path: filepath.Join(t.TempDir(), "config.json"), settings: savedSettings{PlexURL: "https://plex.example", PlexToken: "saved-secret", PollIntervalSeconds: 3}}
	handler := appHandler(store, func() {}, "test-control-token", nil)
	payload := `{"plexUrl":"https://PLEX.example:443/library","plexToken":"","pollIntervalSeconds":5}`
	request := httptest.NewRequest(http.MethodPost, "http://"+listenAddr+"/api/settings", strings.NewReader(payload))
	request.Host = listenAddr
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://"+listenAddr)
	request.Header.Set(controlTokenHeader, "test-control-token")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := store.snapshot(); got.PlexToken != "saved-secret" || got.PollIntervalSeconds != 5 {
		t.Fatalf("settings = %+v", got)
	}
}

func TestValidateSettingsRequiresExplicitRemoteHTTPOptIn(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		allow   bool
		wantErr bool
	}{
		{name: "remote HTTP rejected", url: "http://192.168.1.100:32400", wantErr: true},
		{name: "remote HTTP explicit opt in", url: "http://192.168.1.100:32400", allow: true},
		{name: "loopback HTTP", url: "http://127.0.0.1:32400"},
		{name: "localhost HTTP", url: "http://localhost:32400"},
		{name: "remote HTTPS", url: "https://plex.example"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := validateSettings(savedSettings{PlexURL: test.url, PlexToken: "secret", PollIntervalSeconds: 3, AllowInsecureHTTP: test.allow})
			if (err != nil) != test.wantErr {
				t.Fatalf("validateSettings() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestPlexClientRejectsCrossOriginRedirectBeforeSendingToken(t *testing.T) {
	targetCalled := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetCalled = true
		if token := r.Header.Get("X-Plex-Token"); token != "" {
			t.Errorf("redirect target received token %q", token)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, target.URL, http.StatusFound)
	}))
	defer source.Close()

	client, err := clientFromSettings(savedSettings{PlexURL: source.URL, PlexToken: "secret", PollIntervalSeconds: 3})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.request(context.Background(), "/status/sessions")
	if response != nil {
		response.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "configured origin") {
		t.Fatalf("redirect error = %v", err)
	}
	if targetCalled {
		t.Fatal("cross-origin redirect target was contacted")
	}
}

func TestPlexClientAllowsSameOriginRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/first" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		if got := r.Header.Get("X-Plex-Token"); got != "secret" {
			t.Errorf("token header = %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client, err := clientFromSettings(savedSettings{PlexURL: server.URL, PlexToken: "secret", PollIntervalSeconds: 3})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.request(context.Background(), "/first")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestSecurityHeadersDenyFraming(t *testing.T) {
	handler := appHandler(&settingsStore{}, func() {}, "test-control-token", nil)
	request := httptest.NewRequest(http.MethodGet, "http://"+listenAddr+"/web/setup.html", nil)
	request.Host = listenAddr
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got := response.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q", got)
	}
	if got := response.Header().Get("Content-Security-Policy"); !strings.Contains(got, "frame-ancestors 'none'") {
		t.Fatalf("Content-Security-Policy = %q", got)
	}
}

func TestSettingsRejectCrossOriginWrite(t *testing.T) {
	store, err := newSettingsStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(routes(store, func() {}, "test-control-token", nil))
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
