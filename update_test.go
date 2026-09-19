package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAppVersion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  appVersion
		ok    bool
	}{
		{name: "stable", input: "v1.2.3", want: appVersion{major: 1, minor: 2, patch: 3}, ok: true},
		{name: "without prefix", input: "1.2.3", want: appVersion{major: 1, minor: 2, patch: 3}, ok: true},
		{name: "release candidate", input: "v1.2.3-rc.4", want: appVersion{major: 1, minor: 2, patch: 3, rc: 4}, ok: true},
		{name: "surrounding space", input: " v0.2.1 ", want: appVersion{minor: 2, patch: 1}, ok: true},
		{name: "development", input: "development", ok: false},
		{name: "empty", input: "", ok: false},
		{name: "partial", input: "v1.2", ok: false},
		{name: "beta", input: "v1.2.3-beta.1", ok: false},
		{name: "trailing text", input: "v1.2.3-rc.1-extra", ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := parseAppVersion(test.input)
			if ok != test.ok {
				t.Fatalf("ok = %v, want %v", ok, test.ok)
			}
			if ok && got != test.want {
				t.Fatalf("version = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestCompareAppVersions(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "equal", a: "v1.2.3", b: "v1.2.3", want: 0},
		{name: "patch newer", a: "v1.2.4", b: "v1.2.3", want: 1},
		{name: "minor newer", a: "v1.3.0", b: "v1.2.9", want: 1},
		{name: "major newer", a: "v2.0.0", b: "v1.9.9", want: 1},
		{name: "older", a: "v1.2.3", b: "v1.2.4", want: -1},
		{name: "stable beats rc", a: "v1.2.3", b: "v1.2.3-rc.9", want: 1},
		{name: "rc loses to stable", a: "v1.2.3-rc.9", b: "v1.2.3", want: -1},
		{name: "later rc newer", a: "v1.2.3-rc.2", b: "v1.2.3-rc.1", want: 1},
		{name: "earlier rc older", a: "v1.2.3-rc.1", b: "v1.2.3-rc.2", want: -1},
		{name: "rc of newer patch wins", a: "v1.2.4-rc.1", b: "v1.2.3", want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a, ok := parseAppVersion(test.a)
			if !ok {
				t.Fatalf("cannot parse %q", test.a)
			}
			b, ok := parseAppVersion(test.b)
			if !ok {
				t.Fatalf("cannot parse %q", test.b)
			}
			if got := compareAppVersions(a, b); got != test.want {
				t.Fatalf("compare(%s, %s) = %d, want %d", test.a, test.b, got, test.want)
			}
		})
	}
}

func releaseFixture(tag string, prerelease, draft bool, assets ...string) githubRelease {
	release := githubRelease{TagName: tag, Prerelease: prerelease, Draft: draft, HTMLURL: "https://github.com/example/releases/" + tag}
	for _, name := range assets {
		release.Assets = append(release.Assets, githubAsset{Name: name, BrowserDownloadURL: "https://github.com/example/" + name})
	}
	return release
}

func TestSelectUpdateChoosesNewestEligibleRelease(t *testing.T) {
	complete := []string{updateAssetName, checksumAssetName}
	tests := []struct {
		name               string
		current            string
		includePrereleases bool
		releases           []githubRelease
		wantAvailable      bool
		wantLatest         string
		wantErr            bool
	}{
		{
			name:    "newer stable available",
			current: "v0.2.1",
			releases: []githubRelease{
				releaseFixture("v0.2.2", false, false, complete...),
				releaseFixture("v0.2.1", false, false, complete...),
			},
			wantAvailable: true,
			wantLatest:    "v0.2.2",
		},
		{
			name:    "already current",
			current: "v0.2.1",
			releases: []githubRelease{
				releaseFixture("v0.2.1", false, false, complete...),
			},
			wantAvailable: false,
		},
		{
			name:    "older release ignored",
			current: "v0.3.0",
			releases: []githubRelease{
				releaseFixture("v0.2.9", false, false, complete...),
			},
			wantAvailable: false,
		},
		{
			name:    "draft ignored",
			current: "v0.2.1",
			releases: []githubRelease{
				releaseFixture("v0.3.0", false, true, complete...),
			},
			wantAvailable: false,
		},
		{
			name:    "prerelease skipped by default",
			current: "v0.2.1",
			releases: []githubRelease{
				releaseFixture("v0.3.0-rc.1", true, false, complete...),
			},
			wantAvailable: false,
		},
		{
			name:               "prerelease included when opted in",
			current:            "v0.2.1",
			includePrereleases: true,
			releases: []githubRelease{
				releaseFixture("v0.3.0-rc.1", true, false, complete...),
			},
			wantAvailable: true,
			wantLatest:    "v0.3.0-rc.1",
		},
		{
			name:               "stable preferred over prerelease",
			current:            "v0.2.1",
			includePrereleases: true,
			releases: []githubRelease{
				releaseFixture("v0.3.0-rc.2", true, false, complete...),
				releaseFixture("v0.3.0", false, false, complete...),
			},
			wantAvailable: true,
			wantLatest:    "v0.3.0",
		},
		{
			name:               "newer prerelease beats older stable",
			current:            "v0.2.1",
			includePrereleases: true,
			releases: []githubRelease{
				releaseFixture("v0.3.0-rc.1", true, false, complete...),
				releaseFixture("v0.2.5", false, false, complete...),
			},
			wantAvailable: true,
			wantLatest:    "v0.3.0-rc.1",
		},
		{
			name:    "missing binary asset",
			current: "v0.2.1",
			releases: []githubRelease{
				releaseFixture("v0.2.2", false, false, checksumAssetName),
			},
			wantErr: true,
		},
		{
			name:    "missing checksum asset",
			current: "v0.2.1",
			releases: []githubRelease{
				releaseFixture("v0.2.2", false, false, updateAssetName),
			},
			wantErr: true,
		},
		{
			name:    "development build cannot update",
			current: "development",
			releases: []githubRelease{
				releaseFixture("v0.2.2", false, false, complete...),
			},
			wantErr: true,
		},
		{
			name:    "unparseable tags ignored",
			current: "v0.2.1",
			releases: []githubRelease{
				releaseFixture("nightly", false, false, complete...),
			},
			wantAvailable: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info, err := selectUpdate(test.current, test.releases, test.includePrereleases)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if info.Available != test.wantAvailable {
				t.Fatalf("available = %v, want %v", info.Available, test.wantAvailable)
			}
			if info.Latest != test.wantLatest {
				t.Fatalf("latest = %q, want %q", info.Latest, test.wantLatest)
			}
			if info.Available && (info.downloadURL == "" || info.checksumURL == "") {
				t.Fatalf("available update is missing download addresses: %+v", info)
			}
		})
	}
}

func TestSelectUpdateTruncatesLongNotes(t *testing.T) {
	release := releaseFixture("v0.2.2", false, false, updateAssetName, checksumAssetName)
	release.Body = strings.Repeat("a", maxReleaseNotes+500)
	info, err := selectUpdate("v0.2.1", []githubRelease{release}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Notes) > maxReleaseNotes+4 {
		t.Fatalf("notes were not truncated: %d characters", len(info.Notes))
	}
}

func TestAllowedUpdateHost(t *testing.T) {
	tests := []struct {
		address string
		want    bool
	}{
		{address: "https://api.github.com/repos/x/y/releases", want: true},
		{address: "https://github.com/x/y/releases/download/v1/app.exe", want: true},
		{address: "https://objects.githubusercontent.com/asset", want: true},
		{address: "https://release-assets.githubusercontent.com/asset", want: true},
		{address: "http://github.com/x/y", want: false},
		{address: "https://attacker.example/app.exe", want: false},
		{address: "https://github.com.attacker.example/app.exe", want: false},
	}
	for _, test := range tests {
		t.Run(test.address, func(t *testing.T) {
			parsed, err := url.Parse(test.address)
			if err != nil {
				t.Fatal(err)
			}
			if got := allowedUpdateHost(parsed); got != test.want {
				t.Fatalf("allowedUpdateHost(%s) = %v, want %v", test.address, got, test.want)
			}
		})
	}
}

func TestReleaseClientRejectsRedirectOffAllowedHosts(t *testing.T) {
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("payload"))
	}))
	defer attacker.Close()

	client := newReleaseClient()
	request, err := http.NewRequest(http.MethodGet, attacker.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.client.CheckRedirect(request, []*http.Request{request}); err == nil {
		t.Fatal("expected a redirect off the allowed hosts to be rejected")
	}
}

func TestReleaseClientDownloadVerified(t *testing.T) {
	payload := []byte("new binary contents")
	digest := sha256.Sum256(payload)
	checksumBody := hex.EncodeToString(digest[:]) + "  " + updateAssetName + "\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/checksum":
			_, _ = w.Write([]byte(checksumBody))
		case "/binary":
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newReleaseClient()
	destination := filepath.Join(t.TempDir(), "staged.exe")
	info := updateInfo{downloadURL: server.URL + "/binary", checksumURL: server.URL + "/checksum"}
	if err := client.downloadVerified(context.Background(), info, destination); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("staged file = %q", got)
	}
}

func TestReleaseClientRejectsChecksumMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/checksum":
			_, _ = w.Write([]byte(strings.Repeat("0", 64) + "  " + updateAssetName + "\n"))
		case "/binary":
			_, _ = w.Write([]byte("tampered contents"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newReleaseClient()
	destination := filepath.Join(t.TempDir(), "staged.exe")
	info := updateInfo{downloadURL: server.URL + "/binary", checksumURL: server.URL + "/checksum"}
	if err := client.downloadVerified(context.Background(), info, destination); err == nil {
		t.Fatal("expected a checksum mismatch to be rejected")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("rejected download was not removed: %v", err)
	}
}

func TestReleaseClientRejectsMalformedChecksum(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not-a-digest\n"))
	}))
	defer server.Close()

	client := newReleaseClient()
	if _, err := client.checksum(context.Background(), server.URL); err == nil {
		t.Fatal("expected a malformed checksum to be rejected")
	}
}

func TestUpdateEndpointsRequireControlAuthorization(t *testing.T) {
	store := &settingsStore{settings: savedSettings{PollIntervalSeconds: 3}}
	handler := appHandler(store, func() {}, "test-control-token", newUpdateManager(store))
	tests := []struct {
		name   string
		path   string
		origin string
		token  string
		want   int
	}{
		{name: "check without origin", path: "/api/update/check", token: "test-control-token", want: http.StatusForbidden},
		{name: "check with rebound origin", path: "/api/update/check", origin: "http://attacker.example:7070", token: "test-control-token", want: http.StatusForbidden},
		{name: "check without token", path: "/api/update/check", origin: "http://" + listenAddr, want: http.StatusForbidden},
		{name: "check with wrong token", path: "/api/update/check", origin: "http://" + listenAddr, token: "wrong", want: http.StatusForbidden},
		{name: "apply without token", path: "/api/update/apply", origin: "http://" + listenAddr, want: http.StatusForbidden},
		{name: "apply with wrong token", path: "/api/update/apply", origin: "http://" + listenAddr, token: "wrong", want: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://"+listenAddr+test.path, nil)
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

func TestUpdateStatusIsReadableWithoutAuthorization(t *testing.T) {
	store := &settingsStore{settings: savedSettings{PollIntervalSeconds: 3}}
	handler := appHandler(store, func() {}, "test-control-token", newUpdateManager(store))
	request := httptest.NewRequest(http.MethodGet, "http://"+listenAddr+"/api/update/status", nil)
	request.Host = listenAddr
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var status updateStatus
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if status.Checked {
		t.Fatal("a fresh manager should not report a completed check")
	}
	if status.Current != applicationVersion() {
		t.Fatalf("current = %q", status.Current)
	}
}

func TestUpdateStatusNeverExposesDownloadAddresses(t *testing.T) {
	store := &settingsStore{settings: savedSettings{PollIntervalSeconds: 3}}
	manager := newUpdateManager(store)
	manager.record(updateInfo{
		Available:   true,
		Current:     "v0.2.1",
		Latest:      "v0.2.2",
		downloadURL: "https://github.com/secret/binary",
		checksumURL: "https://github.com/secret/checksum",
	}, nil)

	handler := appHandler(store, func() {}, "test-control-token", manager)
	request := httptest.NewRequest(http.MethodGet, "http://"+listenAddr+"/api/update/status", nil)
	request.Host = listenAddr
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	body := response.Body.String()
	if strings.Contains(body, "secret") || strings.Contains(body, "downloadURL") || strings.Contains(body, "checksumURL") {
		t.Fatalf("status leaked internal addresses: %s", body)
	}
}

func TestUpdateManagerStageRequiresAvailableUpdate(t *testing.T) {
	store := &settingsStore{settings: savedSettings{PollIntervalSeconds: 3}}
	manager := newUpdateManager(store)
	if _, err := manager.stage(context.Background()); err == nil {
		t.Fatal("expected staging without an available update to fail")
	}
}

func TestUpdateManagerTakeRestartIsOneShot(t *testing.T) {
	manager := newUpdateManager(&settingsStore{})
	if _, ok := manager.takeRestart(); ok {
		t.Fatal("no restart should be pending")
	}
	manager.requestRestart("C:/app/PlexSongOBSOverlay.exe.new")
	staged, ok := manager.takeRestart()
	if !ok || staged != "C:/app/PlexSongOBSOverlay.exe.new" {
		t.Fatalf("staged = %q, ok = %v", staged, ok)
	}
	if _, ok := manager.takeRestart(); ok {
		t.Fatal("restart should only be reported once")
	}
}

func TestSettingsPersistIncludePrereleases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store, err := newSettingsStore(path)
	if err != nil {
		t.Fatal(err)
	}
	settings := savedSettings{PlexURL: "https://plex.example", PlexToken: "secret", PollIntervalSeconds: 3, IncludePrereleases: true}
	if err := store.save(settings); err != nil {
		t.Fatal(err)
	}
	reloaded, err := newSettingsStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.snapshot().IncludePrereleases {
		t.Fatal("includePrereleases was not persisted")
	}
}
