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
)

var _ http.RoundTripper = &Client{}

func (c *Client) getCachePath(url *url.URL) string {
	name := strings.ReplaceAll(url.Host+"/"+url.Path, "/", "_")
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

type cachingBody struct {
	io.Reader
	httpBody io.Closer
	file     *os.File
}

func (c *cachingBody) Close() error {
	err := errors.Join(c.httpBody.Close(), c.file.Close())
	if err != nil {
		return err
	}

	return os.Rename(c.file.Name(), strings.TrimSuffix(c.file.Name(), ".dl"))
}

func (c *Client) RoundTrip(req *http.Request) (*http.Response, error) {
	req.AddCookie(&http.Cookie{Name: "session", Value: c.sessionCookie})
	req.Header.Set("User-Agent", "github.com/tombl/aoc")

	path := c.getCachePath(req.URL)
	if req.Method == http.MethodGet || req.Method == "" {
		if file, err := os.Open(path); err == nil {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       file,
			}, nil
		}
	}

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	if resp.StatusCode != http.StatusOK {
		return resp, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	file, err := os.Create(path + ".dl")
	if err != nil {
		resp.Body.Close()
		return resp, fmt.Errorf("creating cache file: %w", err)
	}
	resp.Body = &cachingBody{
		Reader:   io.TeeReader(resp.Body, file),
		httpBody: resp.Body, file: file,
	}
	return resp, nil
}
