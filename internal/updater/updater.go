package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var log = slog.With("subsystem", "updater")

// DefaultAPIURL is the GitHub releases API endpoint for this project.
const DefaultAPIURL = "https://api.github.com/repos/soarqin/smtc-now-playing/releases/latest"

// compareVersions returns true if 'a' is newer than 'b'.
// Assumes format "X.Y.Z" with no leading 'v'.
func compareVersions(a, b string) bool {
	aParts := strings.SplitN(a, ".", 3)
	bParts := strings.SplitN(b, ".", 3)
	for i := 0; i < 3; i++ {
		ai, _ := strconv.Atoi(aParts[i])
		bi, _ := strconv.Atoi(bParts[i])
		if ai != bi {
			return ai > bi
		}
	}
	return false // equal
}

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
// The context is used to control the HTTP request timeout and cancellation.
func CheckForUpdate(ctx context.Context, currentVersion, apiURL string) (*UpdateInfo, error) {
	return checkForUpdate(ctx, currentVersion, apiURL, &http.Client{Timeout: 10 * time.Second})
}

// checkForUpdate is the internal implementation that accepts a custom http.Client for testability.
func checkForUpdate(ctx context.Context, currentVersion, apiURL string, client *http.Client) (*UpdateInfo, error) {
	// Apply a 10-second timeout if the context doesn't already have one.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("update check request creation failed: %w", err)
	}

	resp, err := client.Do(req)
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
	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(currentVersion, "v")

	return &UpdateInfo{
		Available:    compareVersions(latest, current),
		Version:      release.TagName,
		URL:          release.HTMLURL,
		ReleaseNotes: release.Body,
	}, nil
}
