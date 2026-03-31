package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DefaultAPIURL is the GitHub releases API endpoint for this project.
const DefaultAPIURL = "https://api.github.com/repos/soarqin/smtc-now-playing/releases/latest"

// UpdateInfo holds information about an available update.
type UpdateInfo struct {
	Available    bool
	Version      string // e.g. "v1.3.0"
	URL          string // GitHub release URL
	ReleaseNotes string
}

// CheckForUpdate queries GitHub releases API and compares with currentVersion.
// Returns nil, nil if up to date. Returns UpdateInfo with Available=true if newer.
// Returns nil, error on network/parse failure.
func CheckForUpdate(currentVersion, apiURL string) (*UpdateInfo, error) {
	return checkForUpdate(currentVersion, apiURL, &http.Client{Timeout: 10 * time.Second})
}

// checkForUpdate is the internal implementation that accepts a custom http.Client for testability.
func checkForUpdate(currentVersion, apiURL string, client *http.Client) (*UpdateInfo, error) {
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("update check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update check returned status %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("update check decode failed: %w", err)
	}

	// Normalize versions: strip leading "v" for comparison.
	// Simple string comparison works for semver x.y.z as long as components are single digits.
	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(currentVersion, "v")

	return &UpdateInfo{
		Available:    latest > current,
		Version:      release.TagName,
		URL:          release.HTMLURL,
		ReleaseNotes: release.Body,
	}, nil
}
