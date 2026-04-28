package runtime

import (
	"encoding/xml"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func parseFeedItems(raw []byte, source models.AgentWorkflowSource) []models.AgentWorkflowItem {
	items := parseRSS(raw, source)
	if len(items) > 0 {
		return items
	}
	return parseAtom(raw, source)
}

func parseRSS(raw []byte, source models.AgentWorkflowSource) []models.AgentWorkflowItem {
	var feed struct {
		Channel struct {
			Items []struct {
				Title       string `xml:"title"`
				Link        string `xml:"link"`
				GUID        string `xml:"guid"`
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
			GUID:        strings.TrimSpace(item.GUID),
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
			ID      string `xml:"id"`
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
			GUID:        strings.TrimSpace(entry.ID),
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
