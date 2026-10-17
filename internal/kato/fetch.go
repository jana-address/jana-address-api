package kato

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

const (
	// DefaultPageURL is the classifier page that links the current file.
	DefaultPageURL = "https://stat.gov.kz/ru/classifiers/statistical/21/"
	UserAgent      = "jana-address/kato-sync"

	maxPageBytes = 2 << 20
	maxFileBytes = 20 << 20
)

var (
	hrefRe = regexp.MustCompile(`(?i)\bhref\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
	dateRe = regexp.MustCompile(`(\d{2})\.(\d{2})\.(\d{4})`)
)

// FindXLSXLink returns the absolute URL of the KATO workbook linked from the
// classifier page: an .xlsx whose file name starts with "КАТО". If several
// match, the one with the latest date in its name wins.
func FindXLSXLink(page []byte, base *url.URL) (*url.URL, error) {
	var best *url.URL
	bestDate := ""
	for _, m := range hrefRe.FindAllSubmatch(page, -1) {
		raw := strings.TrimSpace(html.UnescapeString(string(m[1]) + string(m[2]) + string(m[3])))
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if base != nil {
			u = base.ResolveReference(u)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			continue
		}
		name := path.Base(u.Path)
		if !strings.EqualFold(path.Ext(name), ".xlsx") || !isKATOName(name) {
			continue
		}
		if d := SourceDate(name); best == nil || d > bestDate {
			best, bestDate = u, d
		}
	}
	if best == nil {
		return nil, errors.New("kato: no KATO .xlsx link on the page")
	}
	return best, nil
}

func isKATOName(name string) bool {
	up := strings.ToUpper(name)
	return strings.HasPrefix(up, "КАТО") || strings.HasPrefix(up, "KATO")
}

// SourceDate extracts the publication date from a file name or URL such as
// ".../КАТО_18.09.2026.xlsx" and returns it as YYYY-MM-DD, or "" if absent.
func SourceDate(src string) string {
	if u, err := url.PathUnescape(src); err == nil {
		src = u
	}
	m := dateRe.FindAllStringSubmatch(path.Base(src), -1)
	if m == nil {
		return ""
	}
	last := m[len(m)-1]
	t, err := time.Parse("02.01.2006", last[1]+"."+last[2]+"."+last[3])
	if err != nil {
		return ""
	}
	return t.Format(time.DateOnly)
}

// DisplayURL returns u with the path unescaped, so Cyrillic file names stay
// readable in the manifest.
func DisplayURL(raw string) string {
	if s, err := url.PathUnescape(raw); err == nil {
		return s
	}
	return raw
}

// Fetch downloads the classifier page, follows the KATO link and returns
// the workbook bytes together with the URL they came from.
func Fetch(ctx context.Context, c *http.Client, pageURL string) (data []byte, src string, err error) {
	if c == nil {
		c = &http.Client{Timeout: 5 * time.Minute}
	}
	page, base, err := get(ctx, c, pageURL, maxPageBytes)
	if err != nil {
		return nil, "", err
	}
	link, err := FindXLSXLink(page, base)
	if err != nil {
		return nil, "", err
	}
	data, _, err = get(ctx, c, link.String(), maxFileBytes)
	if err != nil {
		return nil, "", err
	}
	if !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return nil, "", fmt.Errorf("kato: %s is not an xlsx (zip) file", DisplayURL(link.String()))
	}
	return data, DisplayURL(link.String()), nil
}

// get returns the body and the final URL after redirects.
func get(ctx context.Context, c *http.Client, u string, limit int64) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("kato: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := c.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("kato: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("kato: GET %s: %s", DisplayURL(u), resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, nil, fmt.Errorf("kato: GET %s: %w", DisplayURL(u), err)
	}
	if int64(len(b)) > limit {
		return nil, nil, fmt.Errorf("kato: GET %s: body exceeds %d bytes", DisplayURL(u), limit)
	}
	return b, resp.Request.URL, nil
}
