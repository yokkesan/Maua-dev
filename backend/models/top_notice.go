package models

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	noteRSSURL      = "https://note.com/auma4636/rss"
	maxRSSResponse  = 2 * 1024 * 1024
	rssRequestLimit = 5 * time.Second
)

var ErrNoticeNotFound = errors.New("notice not found")

type TopNotice struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	PublishedAt string `json:"publishedAt"`
}

type rss struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	PubDate string `xml:"pubDate"`
	Date    string `xml:"date"`
}

func GetLatestTopNotice(ctx context.Context) (*TopNotice, error) {
	client := &http.Client{
		Timeout: rssRequestLimit,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, noteRSSURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create RSS request: %w", err)
	}

	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch RSS: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch RSS: unexpected status %d", res.StatusCode)
	}

	var feed rss

	if err := xml.NewDecoder(
		io.LimitReader(res.Body, maxRSSResponse),
	).Decode(&feed); err != nil {
		return nil, fmt.Errorf("decode RSS: %w", err)
	}

	if len(feed.Channel.Items) == 0 {
		return nil, ErrNoticeNotFound
	}

	item := feed.Channel.Items[0]

	return &TopNotice{
		Title:       strings.TrimSpace(item.Title),
		URL:         strings.TrimSpace(item.Link),
		PublishedAt: normalizePublishedAt(item),
	}, nil
}

func normalizePublishedAt(item rssItem) string {
	value := strings.TrimSpace(item.PubDate)

	if value == "" {
		value = strings.TrimSpace(item.Date)
	}

	if value == "" {
		return ""
	}

	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
	}

	for _, format := range formats {
		t, err := time.Parse(format, value)
		if err == nil {
			return t.Format(time.RFC3339)
		}
	}

	return value
}