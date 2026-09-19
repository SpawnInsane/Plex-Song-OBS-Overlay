package main

//go:generate go run github.com/tc-hib/go-winres@v0.3.3 simply --arch amd64 --manifest gui --icon assets/plex-song-obs-overlay.png --out rsrc

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	listenAddr         = "127.0.0.1:7070"
	controlTokenHeader = "X-Plex-Overlay-Control-Token"
)

const (
	productName       = "Plex Song OBS Overlay"
	legacyProductName = "Plex Song Grabber"
)

//go:embed web/*
var webFiles embed.FS

//go:embed version.json
var versionData []byte

type savedSettings struct {
	PlexURL             string `json:"plexUrl"`
	PlexToken           string `json:"plexToken"`
	PlexUser            string `json:"plexUser"`
	PollIntervalSeconds int    `json:"pollIntervalSeconds"`
	AllowInsecureHTTP   bool   `json:"allowInsecureHttp,omitempty"`
	IncludePrereleases  bool   `json:"includePrereleases,omitempty"`
}

type settingsStore struct {
	mu       sync.RWMutex
	path     string
	settings savedSettings
}

type plexClient struct {
	base   *url.URL
	token  string
	client *http.Client
}

type mediaContainer struct {
	Tracks []track `xml:"Track"`
}

type track struct {
	Type        string   `xml:"type,attr"`
	RatingKey   string   `xml:"ratingKey,attr"`
	Key         string   `xml:"key,attr"`
	Title       string   `xml:"title,attr"`
	Grandparent string   `xml:"grandparentTitle,attr"`
	Parent      string   `xml:"parentTitle,attr"`
	Thumb       string   `xml:"thumb,attr"`
	Duration    int64    `xml:"duration,attr"`
	ViewOffset  int64    `xml:"viewOffset,attr"`
	User        plexUser `xml:"User"`
	Player      player   `xml:"Player"`
}

type plexUser struct {
	Title string `xml:"title,attr"`
}

type player struct {
	State string `xml:"state,attr"`
}

type nowPlaying struct {
	Playing    bool   `json:"playing"`
	Paused     bool   `json:"paused"`
	TrackID    string `json:"trackId,omitempty"`
	Title      string `json:"title,omitempty"`
	Artist     string `json:"artist,omitempty"`
	Album      string `json:"album,omitempty"`
	PositionMS int64  `json:"positionMs,omitempty"`
	DurationMS int64  `json:"durationMs,omitempty"`
	ArtworkURL string `json:"artworkUrl,omitempty"`
}

type publicSettings struct {
	PlexURL             string `json:"plexUrl"`
	PlexUser            string `json:"plexUser"`
	PollIntervalSeconds int    `json:"pollIntervalSeconds"`
	AllowInsecureHTTP   bool   `json:"allowInsecureHttp"`
	IncludePrereleases  bool   `json:"includePrereleases"`
	TokenConfigured     bool   `json:"tokenConfigured"`
	ConfigPath          string `json:"configPath"`
	Version             string `json:"version"`
}

func applicationVersion() string {
	var metadata struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(versionData, &metadata) != nil || metadata.Version == "" {
		return "development"
	}
	return metadata.Version
}

func configFilePath() (string, error) {
	return namedConfigFilePath(productName)
}

func namedConfigFilePath(name string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user configuration directory: %w", err)
	}
	return filepath.Join(dir, name, "config.json"), nil
}

func newSettingsStore(path string) (*settingsStore, error) {
	store := &settingsStore{path: path, settings: savedSettings{PollIntervalSeconds: 3}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	if err := json.Unmarshal(data, &store.settings); err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	if store.settings.PollIntervalSeconds == 0 {
		store.settings.PollIntervalSeconds = 3
	}
	return store, nil
}

func (s *settingsStore) snapshot() savedSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

func (s *settingsStore) save(next savedSettings) error {
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	s.mu.Lock()
	s.settings = next
	s.mu.Unlock()
	return nil
}

func migrateLegacySettings(current *settingsStore, legacyPath string) error {
	if _, err := os.Stat(current.path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check current settings: %w", err)
	}
	if _, err := os.Stat(legacyPath); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("check legacy settings: %w", err)
	}
	legacy, err := newSettingsStore(legacyPath)
	if err != nil {
		return fmt.Errorf("read legacy settings: %w", err)
	}
	if err := current.save(legacy.snapshot()); err != nil {
		return fmt.Errorf("migrate legacy settings: %w", err)
	}
	return nil
}

func validateSettings(settings savedSettings) (*url.URL, error) {
	base, err := url.Parse(strings.TrimSpace(settings.PlexURL))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, errors.New("enter a valid Plex URL beginning with http:// or https://")
	}
	if strings.TrimSpace(settings.PlexToken) == "" {
		return nil, errors.New("enter a Plex token")
	}
	if base.Scheme == "http" && !isLoopbackHost(base.Hostname()) && !settings.AllowInsecureHTTP {
		return nil, errors.New("use HTTPS for a remote Plex server or explicitly allow insecure HTTP")
	}
	if settings.PollIntervalSeconds < 1 || settings.PollIntervalSeconds > 60 {
		return nil, errors.New("refresh interval must be between 1 and 60 seconds")
	}
	base.Path = strings.TrimRight(base.Path, "/")
	base.RawQuery = ""
	base.Fragment = ""
	return base, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func normalizedOrigin(u *url.URL) string {
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}
	return strings.ToLower(u.Scheme) + "://" + net.JoinHostPort(strings.ToLower(u.Hostname()), port)
}

func samePlexOrigin(a, b *url.URL) bool {
	return normalizedOrigin(a) == normalizedOrigin(b)
}

func clientFromSettings(settings savedSettings) (*plexClient, error) {
	base, err := validateSettings(settings)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 8 * time.Second}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many Plex redirects")
		}
		if !samePlexOrigin(base, req.URL) {
			req.Header.Del("X-Plex-Token")
			return errors.New("Plex redirect left the configured origin")
		}
		return nil
	}
	return &plexClient{base: base, token: settings.PlexToken, client: client}, nil
}

func (p *plexClient) request(ctx context.Context, path string) (*http.Response, error) {
	u := *p.base
	u.Path = strings.TrimRight(p.base.Path, "/") + "/" + strings.TrimLeft(path, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/xml")
	req.Header.Set("X-Plex-Token", p.token)
	req.Header.Set("X-Plex-Product", productName)
	req.Header.Set("X-Plex-Client-Identifier", "plex-song-obs-overlay")
	return p.client.Do(req)
}

func (p *plexClient) sessions(ctx context.Context) (mediaContainer, error) {
	resp, err := p.request(ctx, "/status/sessions")
	if err != nil {
		return mediaContainer{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return mediaContainer{}, fmt.Errorf("Plex returned %s", resp.Status)
	}
	var sessions mediaContainer
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&sessions); err != nil {
		return mediaContainer{}, fmt.Errorf("decode Plex response: %w", err)
	}
	return sessions, nil
}

func (p *plexClient) currentTrack(ctx context.Context, username string) (track, bool, error) {
	sessions, err := p.sessions(ctx)
	if err != nil {
		return track{}, false, err
	}
	for _, candidate := range sessions.Tracks {
		userMatches := username == "" || strings.EqualFold(candidate.User.Title, username)
		if candidate.Type == "track" && userMatches && (candidate.Player.State == "playing" || candidate.Player.State == "paused") {
			return candidate, true, nil
		}
	}
	return track{}, false, nil
}

func main() {
	cleanupPreviousUpdate()
	path, err := configFilePath()
	if err != nil {
		slog.Error("configuration error", "error", err)
		return
	}
	store, err := newSettingsStore(path)
	if err != nil {
		slog.Error("configuration error", "error", err)
		return
	}
	legacyPath, err := namedConfigFilePath(legacyProductName)
	if err != nil {
		slog.Error("legacy configuration path error", "error", err)
		return
	}
	if err := migrateLegacySettings(store, legacyPath); err != nil {
		slog.Error("configuration migration error", "error", err)
		return
	}
	controlToken, err := newControlToken()
	if err != nil {
		slog.Error("control authorization error", "error", err)
		return
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		if serverAlreadyRunning() {
			serverStopped, stopMonitoring := monitorServerShutdown("http://"+listenAddr+"/health", 500*time.Millisecond)
			defer stopMonitoring()
			if err := runControlWindow("http://"+listenAddr+"/", serverStopped); err != nil {
				slog.Error("cannot open control window", "error", err)
				showControlWindowError(err)
			}
			return
		}
		slog.Error("cannot start local server", "error", err)
		return
	}

	shutdown := make(chan struct{})
	var shutdownOnce sync.Once
	updates := newUpdateManager(store)
	handler := appHandler(store, func() { shutdownOnce.Do(func() { close(shutdown) }) }, controlToken, updates)
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server stopped", "error", err)
		}
		shutdownOnce.Do(func() { close(shutdown) })
	}()

	// Check for updates in the background so a slow or offline GitHub never delays the window.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if _, err := updates.check(ctx); err != nil {
			slog.Warn("update check failed", "error", err)
		}
	}()

	slog.Info(productName+" is ready", "settings", "http://"+listenAddr+"/", "overlay", "http://"+listenAddr+"/web/overlay.html")
	if err := runControlWindow("http://"+listenAddr+"/", shutdown); err != nil {
		slog.Error("cannot open control window", "error", err)
		showControlWindowError(err)
	}
	shutdownOnce.Do(func() { close(shutdown) })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)

	// The port is released now, so a relaunched instance starts its own server
	// instead of attaching to this one.
	if staged, ok := updates.takeRestart(); ok {
		if err := applyUpdate(staged); err != nil {
			slog.Error("could not install the update", "error", err)
		}
	}
}

func newControlToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate control token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func appHandler(store *settingsStore, stop func(), controlToken string, updates *updateManager) http.Handler {
	return securityHeaders(canonicalHost(routes(store, stop, controlToken, updates)))
}

func routes(store *settingsStore, stop func(), controlToken string, updates *updateManager) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/web/setup.html", http.StatusTemporaryRedirect)
	})
	mux.Handle("GET /web/", noCache(http.FileServer(http.FS(webFiles))))
	mux.HandleFunc("GET /api/control-token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, map[string]string{"controlToken": controlToken})
	})
	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, _ *http.Request) {
		current := store.snapshot()
		writeJSON(w, http.StatusOK, publicSettings{
			PlexURL: current.PlexURL, PlexUser: current.PlexUser,
			PollIntervalSeconds: current.PollIntervalSeconds,
			AllowInsecureHTTP:   current.AllowInsecureHTTP,
			IncludePrereleases:  current.IncludePrereleases,
			TokenConfigured:     strings.TrimSpace(current.PlexToken) != "", ConfigPath: store.path,
			Version: applicationVersion(),
		})
	})
	mux.HandleFunc("POST /api/settings", func(w http.ResponseWriter, r *http.Request) {
		if !authorizedStateChange(r, controlToken) {
			http.Error(w, "control authorization rejected", http.StatusForbidden)
			return
		}
		var input savedSettings
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid settings"})
			return
		}
		input.PlexURL = strings.TrimSpace(input.PlexURL)
		input.PlexUser = strings.TrimSpace(input.PlexUser)
		input.PlexToken = strings.TrimSpace(input.PlexToken)
		if input.PlexToken == "" {
			current := store.snapshot()
			currentURL, currentErr := url.Parse(strings.TrimSpace(current.PlexURL))
			nextURL, nextErr := url.Parse(input.PlexURL)
			if currentErr != nil || nextErr != nil || currentURL.Host == "" || nextURL.Host == "" || !samePlexOrigin(currentURL, nextURL) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enter the Plex token again when changing the Plex server"})
				return
			}
			input.PlexToken = current.PlexToken
		}
		if _, err := validateSettings(input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := store.save(input); err != nil {
			slog.Error("save settings", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save settings"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
	})
	mux.HandleFunc("POST /api/test-connection", func(w http.ResponseWriter, r *http.Request) {
		if !authorizedStateChange(r, controlToken) {
			http.Error(w, "control authorization rejected", http.StatusForbidden)
			return
		}
		client, err := clientFromSettings(store.snapshot())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
		defer cancel()
		if _, err := client.sessions(ctx); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "connection failed: " + err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"connected": true})
	})
	mux.HandleFunc("GET /api/now-playing", func(w http.ResponseWriter, r *http.Request) {
		settings := store.snapshot()
		client, err := clientFromSettings(settings)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "setup is incomplete"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
		defer cancel()
		current, found, err := client.currentTrack(ctx, settings.PlexUser)
		if err != nil {
			slog.Warn("Plex request failed", "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Plex is unavailable"})
			return
		}
		if !found {
			writeJSON(w, http.StatusOK, nowPlaying{})
			return
		}
		trackID := current.RatingKey
		if trackID == "" {
			trackID = current.Key
		}
		result := nowPlaying{Playing: current.Player.State == "playing", Paused: current.Player.State == "paused", TrackID: trackID, Title: current.Title, Artist: current.Grandparent, Album: current.Parent, PositionMS: current.ViewOffset, DurationMS: current.Duration}
		if current.Thumb != "" {
			result.ArtworkURL = "/api/artwork?path=" + url.QueryEscape(current.Thumb)
		}
		writeJSON(w, http.StatusOK, result)
	})
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, _ *http.Request) {
		seconds := store.snapshot().PollIntervalSeconds
		if seconds < 1 || seconds > 60 {
			seconds = 3
		}
		writeJSON(w, http.StatusOK, map[string]int{"pollIntervalMs": seconds * 1000})
	})
	mux.HandleFunc("GET /api/artwork", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		if path == "" || !strings.HasPrefix(path, "/") || strings.Contains(path, "\\") {
			http.Error(w, "invalid artwork path", http.StatusBadRequest)
			return
		}
		client, err := clientFromSettings(store.snapshot())
		if err != nil {
			http.Error(w, "setup is incomplete", http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
		defer cancel()
		resp, err := client.request(ctx, path)
		if err != nil {
			http.Error(w, "artwork unavailable", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "image/") {
			http.Error(w, "artwork unavailable", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.Header().Set("Cache-Control", "private, max-age=300")
		_, _ = io.Copy(w, io.LimitReader(resp.Body, 10<<20))
	})
	mux.HandleFunc("GET /api/update/status", func(w http.ResponseWriter, _ *http.Request) {
		if updates == nil {
			writeJSON(w, http.StatusOK, updateStatus{updateInfo: updateInfo{Current: applicationVersion()}})
			return
		}
		writeJSON(w, http.StatusOK, updates.status())
	})
	mux.HandleFunc("POST /api/update/check", func(w http.ResponseWriter, r *http.Request) {
		if !authorizedStateChange(r, controlToken) {
			http.Error(w, "control authorization rejected", http.StatusForbidden)
			return
		}
		if updates == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "update checks are unavailable"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		info, err := updates.check(ctx)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "could not check for updates: " + err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, info)
	})
	mux.HandleFunc("POST /api/update/apply", func(w http.ResponseWriter, r *http.Request) {
		if !authorizedStateChange(r, controlToken) {
			http.Error(w, "control authorization rejected", http.StatusForbidden)
			return
		}
		if updates == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "updates are unavailable"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		staged, err := updates.stage(ctx)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		updates.requestRestart(staged)
		writeJSON(w, http.StatusOK, map[string]bool{"restarting": true})
		go func() {
			time.Sleep(150 * time.Millisecond)
			stop()
		}()
	})
	mux.HandleFunc("POST /api/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if !authorizedStateChange(r, controlToken) {
			http.Error(w, "control authorization rejected", http.StatusForbidden)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"stopping": true})
		go func() {
			time.Sleep(150 * time.Millisecond)
			stop()
		}()
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

func authorizedStateChange(r *http.Request, controlToken string) bool {
	if r.Host != listenAddr || r.Header.Get("Origin") != "http://"+listenAddr {
		return false
	}
	provided := r.Header.Get(controlTokenHeader)
	return len(provided) == len(controlToken) && subtle.ConstantTimeCompare([]byte(provided), []byte(controlToken)) == 1
}

func serverAlreadyRunning() bool {
	client := &http.Client{Timeout: time.Second}
	return serverHealthy(client, "http://"+listenAddr+"/health")
}

func serverHealthy(client *http.Client, address string) bool {
	resp, err := client.Get(address)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func monitorServerShutdown(address string, interval time.Duration) (<-chan struct{}, func()) {
	stopped := make(chan struct{})
	cancel := make(chan struct{})
	var cancelOnce sync.Once
	client := &http.Client{Timeout: time.Second}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if !serverHealthy(client, address) {
					close(stopped)
					return
				}
			case <-cancel:
				return
			}
		}
	}()
	return stopped, func() { cancelOnce.Do(func() { close(cancel) }) }
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}

func canonicalHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != listenAddr {
			http.Error(w, "invalid host", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
