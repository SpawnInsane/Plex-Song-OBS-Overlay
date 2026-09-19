package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	releaseOwner = "SpawnInsane"
	releaseRepo  = "Plex-Song-OBS-Overlay"

	updateAssetName   = "PlexSongOBSOverlay.exe"
	checksumAssetName = "PlexSongOBSOverlay.exe.sha256"

	maxReleaseResponse = 1 << 20
	maxChecksumSize    = 4 << 10
	maxUpdateDownload  = 64 << 20
	maxReleaseNotes    = 4000
)

var releaseAPI = "https://api.github.com/repos/" + releaseOwner + "/" + releaseRepo + "/releases"

// appVersion is a parsed vMAJOR.MINOR.PATCH or vMAJOR.MINOR.PATCH-rc.N tag.
// A release candidate has rc > 0 and always sorts below the matching stable release.
type appVersion struct {
	major int
	minor int
	patch int
	rc    int
}

var versionPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-rc\.(\d+))?$`)

func parseAppVersion(value string) (appVersion, bool) {
	matches := versionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if matches == nil {
		return appVersion{}, false
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return appVersion{}, false
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return appVersion{}, false
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return appVersion{}, false
	}
	rc := 0
	if matches[4] != "" {
		rc, err = strconv.Atoi(matches[4])
		if err != nil {
			return appVersion{}, false
		}
	}
	return appVersion{major: major, minor: minor, patch: patch, rc: rc}, true
}

// compareAppVersions returns -1 when a is older than b, 0 when equal, and 1 when newer.
func compareAppVersions(a, b appVersion) int {
	if a.major != b.major {
		return compareInts(a.major, b.major)
	}
	if a.minor != b.minor {
		return compareInts(a.minor, b.minor)
	}
	if a.patch != b.patch {
		return compareInts(a.patch, b.patch)
	}
	// rc == 0 means a stable release, which outranks any release candidate.
	if a.rc == b.rc {
		return 0
	}
	if a.rc == 0 {
		return 1
	}
	if b.rc == 0 {
		return -1
	}
	return compareInts(a.rc, b.rc)
}

func compareInts(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Name       string        `json:"name"`
	Body       string        `json:"body"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	HTMLURL    string        `json:"html_url"`
	Assets     []githubAsset `json:"assets"`
}

// updateInfo describes the outcome of a release check. The download and checksum
// addresses stay unexported so they are never handed to the browser.
type updateInfo struct {
	Available  bool   `json:"available"`
	Current    string `json:"current"`
	Latest     string `json:"latest,omitempty"`
	Prerelease bool   `json:"prerelease"`
	Notes      string `json:"notes,omitempty"`
	ReleaseURL string `json:"releaseUrl,omitempty"`

	downloadURL string
	checksumURL string
}

type updateStatus struct {
	updateInfo
	Checked bool   `json:"checked"`
	Error   string `json:"error,omitempty"`
}

type releaseClient struct {
	client *http.Client
	apiURL string
}

func newReleaseClient() *releaseClient {
	client := &http.Client{Timeout: 30 * time.Second}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many update redirects")
		}
		if !allowedUpdateHost(req.URL) {
			return errors.New("update redirect left the allowed hosts")
		}
		return nil
	}
	return &releaseClient{client: client, apiURL: releaseAPI}
}

// allowedUpdateHost confines update traffic to GitHub over HTTPS. Release assets
// are served from github.com and then redirected to a GitHub asset host.
func allowedUpdateHost(u *url.URL) bool {
	if u.Scheme != "https" {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "api.github.com", "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com":
		return true
	default:
		return false
	}
}

func (c *releaseClient) releases(ctx context.Context) ([]githubRelease, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL+"?per_page=30", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", productName+"/"+applicationVersion())
	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("GitHub returned %s", response.Status)
	}
	var releases []githubRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, maxReleaseResponse)).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode GitHub response: %w", err)
	}
	return releases, nil
}

func (c *releaseClient) checksum(ctx context.Context, address string) ([]byte, error) {
	response, err := c.get(ctx, address)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("checksum download returned %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxChecksumSize))
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return nil, errors.New("checksum file is empty")
	}
	digest, err := hex.DecodeString(fields[0])
	if err != nil || len(digest) != sha256.Size {
		return nil, errors.New("checksum file does not contain a SHA-256 digest")
	}
	return digest, nil
}

func (c *releaseClient) download(ctx context.Context, address, destination string) ([]byte, error) {
	response, err := c.get(ctx, address)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("update download returned %s", response.Status)
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0700)
	if err != nil {
		return nil, err
	}
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(response.Body, maxUpdateDownload+1))
	closeErr := file.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if written > maxUpdateDownload {
		return nil, errors.New("update download is larger than expected")
	}
	return hasher.Sum(nil), nil
}

func (c *releaseClient) get(ctx context.Context, address string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", productName+"/"+applicationVersion())
	return c.client.Do(request)
}

// downloadVerified fetches the release binary and refuses to keep it unless its
// SHA-256 digest matches the checksum published alongside it.
func (c *releaseClient) downloadVerified(ctx context.Context, info updateInfo, destination string) error {
	expected, err := c.checksum(ctx, info.checksumURL)
	if err != nil {
		return err
	}
	actual, err := c.download(ctx, info.downloadURL, destination)
	if err != nil {
		_ = os.Remove(destination)
		return err
	}
	if subtle.ConstantTimeCompare(expected, actual) != 1 {
		_ = os.Remove(destination)
		return errors.New("the downloaded update did not match its published checksum")
	}
	return nil
}

func findAsset(release *githubRelease, name string) (githubAsset, bool) {
	for _, asset := range release.Assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return githubAsset{}, false
}

// selectUpdate picks the newest eligible release. Drafts are always ignored, and
// pre-releases are ignored unless the operator opted in.
func selectUpdate(current string, releases []githubRelease, includePrereleases bool) (updateInfo, error) {
	info := updateInfo{Current: current}
	currentVersion, ok := parseAppVersion(current)
	if !ok {
		return info, errors.New("this build does not report a release version")
	}

	var best *githubRelease
	var bestVersion appVersion
	for index := range releases {
		release := &releases[index]
		if release.Draft {
			continue
		}
		if release.Prerelease && !includePrereleases {
			continue
		}
		version, ok := parseAppVersion(release.TagName)
		if !ok {
			continue
		}
		if best == nil || compareAppVersions(version, bestVersion) > 0 {
			best, bestVersion = release, version
		}
	}
	if best == nil || compareAppVersions(bestVersion, currentVersion) <= 0 {
		return info, nil
	}

	binary, ok := findAsset(best, updateAssetName)
	if !ok {
		return info, fmt.Errorf("release %s does not include %s", best.TagName, updateAssetName)
	}
	checksum, ok := findAsset(best, checksumAssetName)
	if !ok {
		return info, fmt.Errorf("release %s does not include %s", best.TagName, checksumAssetName)
	}

	info.Available = true
	info.Latest = best.TagName
	info.Prerelease = best.Prerelease
	info.Notes = truncateNotes(best.Body)
	info.ReleaseURL = best.HTMLURL
	info.downloadURL = binary.BrowserDownloadURL
	info.checksumURL = checksum.BrowserDownloadURL
	return info, nil
}

func truncateNotes(notes string) string {
	trimmed := strings.TrimSpace(notes)
	if len(trimmed) <= maxReleaseNotes {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:maxReleaseNotes]) + "…"
}

// updateManager owns the cached check result and coordinates staging an update.
type updateManager struct {
	mu       sync.Mutex
	client   *releaseClient
	store    *settingsStore
	info     updateInfo
	checked  bool
	checkErr string
	applying bool
	staged   string
	restart  bool
}

func newUpdateManager(store *settingsStore) *updateManager {
	return &updateManager{client: newReleaseClient(), store: store}
}

func (m *updateManager) check(ctx context.Context) (updateInfo, error) {
	includePrereleases := m.store.snapshot().IncludePrereleases
	releases, err := m.client.releases(ctx)
	if err != nil {
		m.record(updateInfo{Current: applicationVersion()}, err)
		return updateInfo{}, err
	}
	info, err := selectUpdate(applicationVersion(), releases, includePrereleases)
	if err != nil {
		m.record(updateInfo{Current: applicationVersion()}, err)
		return updateInfo{}, err
	}
	m.record(info, nil)
	return info, nil
}

func (m *updateManager) record(info updateInfo, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.info = info
	m.checked = true
	if err != nil {
		m.checkErr = err.Error()
		return
	}
	m.checkErr = ""
}

func (m *updateManager) status() updateStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	info := m.info
	if info.Current == "" {
		info.Current = applicationVersion()
	}
	return updateStatus{updateInfo: info, Checked: m.checked, Error: m.checkErr}
}

// stage downloads and verifies the pending update next to the running binary so
// the later rename stays on the same volume.
func (m *updateManager) stage(ctx context.Context) (string, error) {
	m.mu.Lock()
	if m.applying {
		m.mu.Unlock()
		return "", errors.New("an update is already being installed")
	}
	info := m.info
	m.applying = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.applying = false
		m.mu.Unlock()
	}()

	if !info.Available {
		return "", errors.New("no update is available")
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate the running application: %w", err)
	}
	staged := executable + ".new"
	if err := m.client.downloadVerified(ctx, info, staged); err != nil {
		return "", err
	}
	return staged, nil
}

func (m *updateManager) requestRestart(staged string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.staged = staged
	m.restart = true
}

// takeRestart reports a staged update that should be installed once the local
// server has released its port.
func (m *updateManager) takeRestart() (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.restart {
		return "", false
	}
	m.restart = false
	return m.staged, true
}
