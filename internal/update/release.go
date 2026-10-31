package update

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Repository is where WorldKit's releases are published.
const Repository = "vuenno/WorldKit"

// ErrMissingAsset means a release lacks the installer or its checksum file, so it can't
// be installed automatically.
var ErrMissingAsset = errors.New("release asset missing")

// errUnexpectedURL means GitHub's answer pointed somewhere WorldKit doesn't download from.
var errUnexpectedURL = errors.New("unexpected URL")

// maxReleaseJSONSize caps how much of GitHub's release answer is read (a release with
// its notes is a few kilobytes).
const maxReleaseJSONSize = 1 << 20

// Release is a published WorldKit version with what's needed to install it.
type Release struct {
	Version   Version
	NotesURL  string // the release page on GitHub
	Installer Asset  // WorldKit-Setup-<version>.exe
	Checksum  Asset  // WorldKit-Setup-<version>.exe.sha256
}

// Asset is a file attached to a release.
type Asset struct {
	Name string
	URL  string
	Size int64
}

// Client talks to GitHub's releases API. The zero value is not usable; use NewClient.
type Client struct {
	http      *http.Client
	userAgent string
	// latestURL is the API address of the newest release (drafts and pre-releases excluded).
	latestURL string
	// pagePrefix and downloadPrefix are where release pages and assets must live; anything
	// else in the answer is refused.
	pagePrefix     string
	downloadPrefix string
}

// ClientOptions configure a Client. Only tests change the addresses.
type ClientOptions struct {
	CurrentVersion string // sent in the User-Agent, which GitHub requires
	APIBaseURL     string // default https://api.github.com
	SiteBaseURL    string // default https://github.com
}

// NewClient makes a client for WorldKit's releases.
func NewClient(options ClientOptions) *Client {
	apiBase := strings.TrimSuffix(cmp.Or(options.APIBaseURL, "https://api.github.com"), "/")
	siteBase := strings.TrimSuffix(cmp.Or(options.SiteBaseURL, "https://github.com"), "/")
	return &Client{
		// No overall timeout: downloads can take a while. Requests carry contexts instead.
		http:           &http.Client{},
		userAgent:      "WorldKit/" + cmp.Or(options.CurrentVersion, "dev"),
		latestURL:      apiBase + "/repos/" + Repository + "/releases/latest",
		pagePrefix:     siteBase + "/" + Repository + "/releases/",
		downloadPrefix: siteBase + "/" + Repository + "/releases/download/",
	}
}

// githubRelease is the part of GitHub's release JSON WorldKit reads.
type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name        string `json:"name"`
		Size        int64  `json:"size"`
		DownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Latest asks GitHub for the newest published release.
func (c *Client) Latest(ctx context.Context) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	request, err := c.newRequest(ctx, c.latestURL)
	if err != nil {
		return Release{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := c.http.Do(request)
	if err != nil {
		return Release{}, fmt.Errorf("ask GitHub for the latest release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("ask GitHub for the latest release: %s", response.Status)
	}
	var answer githubRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, maxReleaseJSONSize)).Decode(&answer); err != nil {
		return Release{}, fmt.Errorf("read GitHub's latest release: %w", err)
	}
	return c.releaseFrom(answer)
}

func (c *Client) newRequest(ctx context.Context, url string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("make request for %q: %w", url, err)
	}
	request.Header.Set("User-Agent", c.userAgent)
	return request, nil
}

// releaseFrom checks GitHub's answer and picks out the installer and its checksum.
func (c *Client) releaseFrom(answer githubRelease) (Release, error) {
	version, err := ParseVersion(answer.TagName)
	if err != nil {
		return Release{}, fmt.Errorf("release tag: %w", err)
	}
	if !strings.HasPrefix(answer.HTMLURL, c.pagePrefix) {
		return Release{}, fmt.Errorf("release page %q: %w", answer.HTMLURL, errUnexpectedURL)
	}
	release := Release{Version: version, NotesURL: answer.HTMLURL}
	installerName := InstallerName(version)
	for _, asset := range answer.Assets {
		found := Asset{Name: asset.Name, URL: asset.DownloadURL, Size: asset.Size}
		switch asset.Name {
		case installerName:
			release.Installer = found
		case installerName + ".sha256":
			release.Checksum = found
		}
	}
	if release.Installer.URL == "" || release.Checksum.URL == "" {
		return Release{}, fmt.Errorf("release %s: %w (needs %s and its .sha256)", version, ErrMissingAsset, installerName)
	}
	for _, asset := range []Asset{release.Installer, release.Checksum} {
		if !strings.HasPrefix(asset.URL, c.downloadPrefix) {
			return Release{}, fmt.Errorf("asset %q: %w", asset.URL, errUnexpectedURL)
		}
	}
	return release, nil
}

// InstallerName is the installer file the release workflow publishes for version.
func InstallerName(version Version) string {
	return "WorldKit-Setup-" + version.String() + ".exe"
}
