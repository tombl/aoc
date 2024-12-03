package aoc

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var _ http.RoundTripper = &Client{}

func (c *Client) getCachePath(url *url.URL) string {
	name := strings.ReplaceAll(url.Host+url.Path, "/", "_")
	if name == "" {
		name = "index.html"
	}
	return filepath.Join(c.cacheDir, name)
}

func (c *Client) invalidate(req *http.Request) error {
	path := c.getCachePath(req.URL)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing cache file: %w", err)
	}
	return nil
}

func (c *Client) RoundTrip(req *http.Request) (*http.Response, error) {
	req.AddCookie(&http.Cookie{Name: "session", Value: c.sessionCookie})
	req.Header.Set("User-Agent", "github.com/tombl/aoc")

	if !(req.Method == http.MethodGet || req.Method == "") {
		return http.DefaultTransport.RoundTrip(req)
	}

	path := c.getCachePath(req.URL)
	stat, err := os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("checking cache file: %w", err)
	}

	now := time.Now().UTC()
	latestRelease := now.Truncate(24 * time.Hour).Add(5 * time.Hour)
	if now.Before(latestRelease) {
		latestRelease = latestRelease.Add(-24 * time.Hour)
	}

	if err != nil || stat.ModTime().Before(latestRelease) {
		resp, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			return resp, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return resp, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		tempPath := path + ".dl"

		file, err := os.Create(tempPath)
		if err != nil {
			return resp, fmt.Errorf("creating cache file: %w", err)
		}
		defer file.Close()

		if _, err := io.Copy(file, resp.Body); err != nil {
			return resp, fmt.Errorf("writing cache file: %w", err)
		}

		if err := os.Rename(tempPath, path); err != nil {
			return resp, fmt.Errorf("renaming cache file: %w", err)
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening cache file: %w", err)
	}

	return &http.Response{StatusCode: http.StatusOK, Body: file}, nil
}
