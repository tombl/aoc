package aoc

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
)

var SessionCookieRegex = regexp.MustCompile("^(session=)?[0-9a-f]{128}$")

type Client struct {
	sessionCookie string
	cacheDir      string
}

func NewClient(sessionCookie string, cacheDir string) (*Client, error) {
	if !SessionCookieRegex.MatchString(sessionCookie) {
		return nil, fmt.Errorf("invalid session cookie")
	}
	sessionCookie = strings.TrimPrefix(sessionCookie, "session=")
	if err := os.Mkdir(cacheDir, 0755); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("creating cache directory: %w", err)
	}
	return &Client{
		sessionCookie: sessionCookie,
		cacheDir:      cacheDir,
	}, nil
}

func (c *Client) Invalidate(url string) error {
	url = "adventofcode.com/" + url
	cacheFile := filepath.Join(c.cacheDir, url)
	if strings.HasSuffix(url, "/") {
		cacheFile = filepath.Join(c.cacheDir, url, "index.html")
	}
	if err := os.Remove(cacheFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing cache file: %w", err)
	}
	return nil
}

func (c *Client) Get(url string) (*goquery.Document, error) {
	url = "adventofcode.com/" + url

	cacheFile := filepath.Join(c.cacheDir, url)
	if strings.HasSuffix(url, "/") {
		cacheFile = filepath.Join(c.cacheDir, url, "index.html")
	}

	// TODO: check if cache file is older than latest release
	if file, err := os.Open(cacheFile); err == nil {
		defer file.Close()
		doc, err := goquery.NewDocumentFromReader(file)
		if err == nil {
			return doc, nil
		}
	}

	req, err := http.NewRequest("GET", "http://"+url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.AddCookie(&http.Cookie{Name: "session", Value: c.sessionCookie})
	req.Header.Set("User-Agent", "github.com/tombl/aoc")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	body := bytes.NewBuffer(bodyBytes)
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	body = bytes.NewBuffer(bodyBytes)
	resp.Body = io.NopCloser(body)
	data, err := httputil.DumpResponse(resp, true)
	if err != nil {
		return nil, fmt.Errorf("dumping response: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		return nil, fmt.Errorf("creating cache directory: %w", err)
	}
	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return nil, fmt.Errorf("writing cache file: %w", err)
	}

	return doc, nil
}

func (c *Client) InvalidateUser() error {
	return c.Invalidate("")
}

func (c *Client) GetUser() (string, error) {
	doc, err := c.Get("")
	if err != nil {
		return "", err
	}

	return doc.Find(".user").Text(), nil
}

func (c *Client) InvalidateDay(year, day int) error {
	return c.Invalidate(fmt.Sprintf("%d/day/%d", year, day))
}

type Day struct {
	Part1, Part2 string
}

func (c *Client) GetDay(year, day int) (*Day, error) {
	doc, err := c.Get(fmt.Sprintf("%d/day/%d", year, day))
	if err != nil {
		return nil, err
	}

	data := &Day{}

	desc := doc.Find(".day-desc")
	switch desc.Length() {
	case 2:
		data.Part2, err = desc.Clone().Eq(1).Html()
		if err != nil {
			panic(fmt.Errorf("getting part 2 description: %w", err))
		}
		fallthrough
	case 1:
		data.Part1, err = desc.Clone().Eq(0).Html()
		if err != nil {
			panic(fmt.Errorf("getting part 1 description: %w", err))
		}
	default:
		panic(fmt.Errorf("unexpected number of descriptions: %d", desc.Length()))
	}

	data.Part1, err = htmltomarkdown.ConvertString(data.Part1)
	if err != nil {
		return nil, fmt.Errorf("converting part 1 description: %w", err)
	}
	data.Part2, err = htmltomarkdown.ConvertString(data.Part2)
	if err != nil {
		return nil, fmt.Errorf("converting part 2 description: %w", err)
	}

	return data, nil
}

func (c *Client) GetInput(year, day, part int) (string, error) {
	panic("not implemented")
}
