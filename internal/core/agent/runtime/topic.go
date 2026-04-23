package runtime

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (s *Service) SaveTopic(ctx context.Context, topic models.AgentTopicSnapshot) (models.AgentSnapshot, error) {
	return s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		now := time.Now().UTC()
		for idx := range topic.Profiles {
			topic.Profiles[idx].ID = firstNonEmpty(topic.Profiles[idx].ID, uuid.NewString())
			topic.Profiles[idx].UpdatedAt = now
			for sourceIdx := range topic.Profiles[idx].Sources {
				source := &topic.Profiles[idx].Sources[sourceIdx]
				source.ID = firstNonEmpty(source.ID, uuid.NewString())
				if source.Weight <= 0 {
					source.Weight = 1
				}
			}
		}
		if topic.ActiveProfileID == "" && len(topic.Profiles) > 0 {
			topic.ActiveProfileID = topic.Profiles[0].ID
		}
		topic.UpdatedAt = now
		snapshot.TopicSummary = topic
		snapshot.UpdatedAt = now
		return nil
	})
}

func (s *Service) RunTopicSummary(ctx context.Context, profileID string) (models.AgentTopicRun, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentTopicRun{}, err
	}
	profile, ok := selectTopicProfile(snapshot.TopicSummary, profileID)
	if !ok {
		return models.AgentTopicRun{}, errors.New("topic profile not found")
	}
	run := models.AgentTopicRun{
		ID:        uuid.NewString(),
		ProfileID: profile.ID,
		CreatedAt: time.Now().UTC(),
		Items:     []models.AgentTopicItem{},
	}
	seen := topicSentLogSet(snapshot.TopicSummary.SentLog)
	for _, source := range profile.Sources {
		if !source.Enabled {
			continue
		}
		items, fetchErr := fetchFeed(ctx, source)
		if fetchErr != nil {
			run.FetchErrors = append(run.FetchErrors, models.AgentRunError{Target: source.ID, Error: fetchErr.Error()})
			continue
		}
		for _, item := range items {
			key := normalizeTopicURL(item.URL)
			if key != "" {
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
			}
			run.Items = append(run.Items, item)
		}
	}
	run.Items = truncateList(run.Items, 30)
	run.Summary = topicFallbackSummary(profile, run)
	if len(run.Items) > 0 {
		if llmSummary, llmErr := s.GenerateText(ctx, buildTopicPrompt(run)); llmErr == nil {
			run.Summary = llmSummary
		}
	}
	_, err = s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		snapshot.TopicSummary.Runs = append([]models.AgentTopicRun{run}, snapshot.TopicSummary.Runs...)
		snapshot.TopicSummary.Runs = truncateList(snapshot.TopicSummary.Runs, 50)
		for _, item := range run.Items {
			if normalized := normalizeTopicURL(item.URL); normalized != "" {
				snapshot.TopicSummary.SentLog = append([]models.AgentTopicSentLogItem{{
					URLNormalized: normalized,
					SentAt:        run.CreatedAt,
					Title:         item.Title,
				}}, snapshot.TopicSummary.SentLog...)
			}
		}
		snapshot.TopicSummary.SentLog = truncateList(snapshot.TopicSummary.SentLog, 1000)
		snapshot.TopicSummary.UpdatedAt = time.Now().UTC()
		snapshot.UpdatedAt = snapshot.TopicSummary.UpdatedAt
		return nil
	})
	return run, err
}

func topicSentLogSet(items []models.AgentTopicSentLogItem) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		if normalized := normalizeTopicURL(item.URLNormalized); normalized != "" {
			out[normalized] = struct{}{}
		}
	}
	return out
}

func normalizeTopicURL(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return ""
	}
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimSuffix(value, "/")
	return value
}

func selectTopicProfile(snapshot models.AgentTopicSnapshot, profileID string) (models.AgentTopicProfile, bool) {
	target := firstNonEmpty(profileID, snapshot.ActiveProfileID)
	for _, profile := range snapshot.Profiles {
		if profile.ID == target {
			return profile, true
		}
	}
	if len(snapshot.Profiles) > 0 && target == "" {
		return snapshot.Profiles[0], true
	}
	return models.AgentTopicProfile{}, false
}

func fetchFeed(ctx context.Context, source models.AgentTopicSource) ([]models.AgentTopicItem, error) {
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

func parseRSS(raw []byte, source models.AgentTopicSource) []models.AgentTopicItem {
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
	items := make([]models.AgentTopicItem, 0, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		items = append(items, models.AgentTopicItem{
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

func parseAtom(raw []byte, source models.AgentTopicSource) []models.AgentTopicItem {
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
	items := make([]models.AgentTopicItem, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		link := ""
		if len(entry.Links) > 0 {
			link = entry.Links[0].Href
		}
		items = append(items, models.AgentTopicItem{
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

func topicFallbackSummary(profile models.AgentTopicProfile, run models.AgentTopicRun) string {
	if len(run.Items) == 0 {
		return "No feed items were selected for profile " + profile.Name + "."
	}
	return "Fetched " + intString(len(run.Items)) + " feed items for profile " + profile.Name + "."
}

func buildTopicPrompt(run models.AgentTopicRun) string {
	var b strings.Builder
	b.WriteString("Summarize these RSS items for a concise operator digest:\n")
	for idx, item := range run.Items {
		b.WriteString(intString(idx + 1))
		b.WriteString(". ")
		b.WriteString(item.Title)
		if item.URL != "" {
			b.WriteString(" - ")
			b.WriteString(item.URL)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func stripXMLText(value string) string {
	value = strings.ReplaceAll(value, "<![CDATA[", "")
	value = strings.ReplaceAll(value, "]]>", "")
	return strings.TrimSpace(value)
}
