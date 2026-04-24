package runtime

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func fetchFeed(ctx context.Context, source models.AgentWorkflowSource) ([]models.AgentWorkflowItem, error) {
	if strings.TrimSpace(source.FeedURL) == "" {
		return nil, errors.New("feed_url is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.FeedURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("feed request failed with " + resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	items := parseRSS(raw, source)
	if len(items) == 0 {
		items = parseAtom(raw, source)
	}
	return items, nil
}

func parseRSS(raw []byte, source models.AgentWorkflowSource) []models.AgentWorkflowItem {
	var feed struct {
		Channel struct {
			Items []struct {
				Title       string `xml:"title"`
				Link        string `xml:"link"`
				PubDate     string `xml:"pubDate"`
				Description string `xml:"description"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(raw, &feed); err != nil {
		return nil
	}
	items := make([]models.AgentWorkflowItem, 0, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		items = append(items, models.AgentWorkflowItem{
			Title:       strings.TrimSpace(item.Title),
			URL:         strings.TrimSpace(item.Link),
			SourceID:    source.ID,
			SourceName:  source.Name,
			PublishedAt: strings.TrimSpace(item.PubDate),
			Summary:     strings.TrimSpace(stripXMLText(item.Description)),
		})
	}
	return items
}

func parseAtom(raw []byte, source models.AgentWorkflowSource) []models.AgentWorkflowItem {
	var feed struct {
		Entries []struct {
			Title   string `xml:"title"`
			Updated string `xml:"updated"`
			Summary string `xml:"summary"`
			Links   []struct {
				Href string `xml:"href,attr"`
			} `xml:"link"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal(raw, &feed); err != nil {
		return nil
	}
	items := make([]models.AgentWorkflowItem, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		link := ""
		if len(entry.Links) > 0 {
			link = entry.Links[0].Href
		}
		items = append(items, models.AgentWorkflowItem{
			Title:       strings.TrimSpace(entry.Title),
			URL:         strings.TrimSpace(link),
			SourceID:    source.ID,
			SourceName:  source.Name,
			PublishedAt: strings.TrimSpace(entry.Updated),
			Summary:     strings.TrimSpace(stripXMLText(entry.Summary)),
		})
	}
	return items
}

func stripXMLText(value string) string {
	value = strings.ReplaceAll(value, "<![CDATA[", "")
	value = strings.ReplaceAll(value, "]]>", "")
	return strings.TrimSpace(value)
}
