package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrCorruptDownload means the installer that arrived isn't the one the release lists:
// wrong size or wrong SHA-256.
var ErrCorruptDownload = errors.New("download doesn't match the release")

const (
	// maxInstallerSize refuses absurd sizes (the installer is a few megabytes).
	maxInstallerSize = 500 << 20
	// maxChecksumSize is enough for "<64 hex digits>  <file name>".
	maxChecksumSize = 4 << 10
	downloadTimeout = 10 * time.Minute
)

// Download fetches release's installer into dir (created if needed, emptied first) and
// returns its path once its size and SHA-256 match the release.
func (c *Client) Download(ctx context.Context, release Release, dir string) (string, error) {
	installer := release.Installer
	if installer.Size <= 0 || installer.Size > maxInstallerSize {
		return "", fmt.Errorf("installer size %d: %w", installer.Size, ErrCorruptDownload)
	}
	if filepath.Base(installer.Name) != installer.Name {
		return "", fmt.Errorf("installer name %q: %w", installer.Name, errUnexpectedURL)
	}
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	wantSum, err := c.fetchChecksum(ctx, release.Checksum)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("clear update folder %q: %w", dir, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("make update folder %q: %w", dir, err)
	}
	path := filepath.Join(dir, installer.Name)
	if err := c.fetchVerified(ctx, installer, wantSum, path); err != nil {
		return "", err
	}
	return path, nil
}

// fetchChecksum reads a sha256sum-style file: the hex digest, then optionally the name.
func (c *Client) fetchChecksum(ctx context.Context, asset Asset) ([]byte, error) {
	body, err := c.get(ctx, asset.URL)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	content, err := io.ReadAll(io.LimitReader(body, maxChecksumSize))
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", asset.Name, err)
	}
	return parseChecksum(content)
}

func parseChecksum(content []byte) ([]byte, error) {
	fields := strings.Fields(string(content))
	if len(fields) == 0 {
		return nil, fmt.Errorf("checksum file is empty: %w", ErrCorruptDownload)
	}
	sum, err := hex.DecodeString(fields[0])
	if err != nil || len(sum) != sha256.Size {
		return nil, fmt.Errorf("checksum %q isn't a SHA-256: %w", fields[0], ErrCorruptDownload)
	}
	return sum, nil
}

// fetchVerified downloads asset to path through a ".partial" file, which is renamed only
// once the size and hash match, so a half or wrong download is never left to run.
func (c *Client) fetchVerified(ctx context.Context, asset Asset, wantSum []byte, path string) error {
	body, err := c.get(ctx, asset.URL)
	if err != nil {
		return err
	}
	defer body.Close()
	partial := path + ".partial"
	file, err := os.Create(partial)
	if err != nil {
		return fmt.Errorf("create %q: %w", partial, err)
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(body, asset.Size+1))
	closeErr := file.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return errors.Join(fmt.Errorf("download %s: %w", asset.Name, err), os.Remove(partial))
	}
	if written != asset.Size || !bytes.Equal(hash.Sum(nil), wantSum) {
		err := fmt.Errorf("%s: got %d bytes, want %d: %w", asset.Name, written, asset.Size, ErrCorruptDownload)
		return errors.Join(err, os.Remove(partial))
	}
	if err := os.Rename(partial, path); err != nil {
		return fmt.Errorf("finish download %q: %w", path, err)
	}
	return nil
}

// get starts a download from one of the release's own URLs.
func (c *Client) get(ctx context.Context, url string) (io.ReadCloser, error) {
	if !strings.HasPrefix(url, c.downloadPrefix) {
		return nil, fmt.Errorf("download %q: %w", url, errUnexpectedURL)
	}
	request, err := c.newRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download %q: %w", url, err)
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close() // the status is the error worth reporting
		return nil, fmt.Errorf("download %q: %s", url, response.Status)
	}
	return response.Body, nil
}
