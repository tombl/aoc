package aoc

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
)

var Timezone *time.Location = time.FixedZone("UTC-5", -5*60*60)

func newRequest(method string, pathname string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, "https://adventofcode.com/"+pathname, body)
	if err != nil {
		panic(err)
	}
	return req
}
func newGetRequest(pathname string) *http.Request {
	return newRequest(http.MethodGet, pathname, nil)
}

var SessionCookieRegex = regexp.MustCompile("^(session=)?[0-9a-f]{128}$")

type Client struct {
	sessionCookie string
	cacheDir      string

	Spinner bool
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

func (c *Client) InvalidateUser() error {
	return c.invalidate(newGetRequest(""))
}

func (c *Client) GetUser() (string, error) {
	resp, err := c.request(newGetRequest(""))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	return doc.Find(".user").Text(), nil
}

func (c *Client) InvalidateDay(year, day int) error {
	return c.invalidate(newGetRequest(fmt.Sprintf("%d/day/%d", year, day)))
}

type Day struct {
	Part1, Part2 string
}

func (c *Client) GetDay(year, day int) (*Day, error) {
	resp, err := c.request(newGetRequest(fmt.Sprintf("%d/day/%d", year, day)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
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

func (c *Client) GetInput(year, day int) (io.ReadCloser, error) {
	resp, err := c.request(newGetRequest(fmt.Sprintf("%d/day/%d/input", year, day)))
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (c *Client) GetExample(year, day, part int) (io.ReadCloser, error) {
	resp, err := c.request(newGetRequest(fmt.Sprintf("%d/day/%d", year, day)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	var example *goquery.Selection
	for part > 0 {
		part--
		example = doc.Find(".day-desc").Eq(part).Find("pre")
		if example.Length() > 0 {
			break
		}
	}

	var longest string
	example.Each(func(i int, s *goquery.Selection) {
		if s.Text() > longest {
			longest = s.Text()
		}
	})
	return io.NopCloser(strings.NewReader(longest)), nil
}

func (c *Client) SubmitAnswer(year, day, part int, answer string) (string, error) {
	if err := c.InvalidateDay(year, day); err != nil {
		return "", fmt.Errorf("invalidating day: %w", err)
	}

	data := url.Values{
		"level":  {strconv.Itoa(part)},
		"answer": {answer},
	}

	req := newRequest(
		http.MethodPost,
		fmt.Sprintf("%d/day/%d/answer", year, day),
		strings.NewReader(data.Encode()),
	)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.request(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	return doc.Find("main").Text(), nil
}
