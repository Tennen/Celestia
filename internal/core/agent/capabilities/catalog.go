package capabilities

import (
	"sort"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func List() []models.AgentCapabilityInfo {
	items := builtinCapabilities()
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return cloneCapabilities(items)
}

func Get(name string) (models.AgentCapabilityInfo, bool) {
	target := NormalizeName(name)
	for _, item := range builtinCapabilities() {
		if NormalizeName(item.Name) == target {
			return cloneCapability(item), true
		}
	}
	return models.AgentCapabilityInfo{}, false
}

func NormalizeName(name string) string {
	value := strings.ToLower(strings.TrimSpace(name))
	value = strings.TrimPrefix(value, "agent.")
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func builtinCapabilities() []models.AgentCapabilityInfo {
	return []models.AgentCapabilityInfo{
		{
			Name:        "apple-notes",
			Description: "Manage Apple Notes via the memo CLI on macOS.",
			Terminal:    true,
			Command:     "memo",
			Install:     "brew tap antoniorodr/memo && brew install antoniorodr/memo/memo",
			Keywords:    []string{"note", "memo", "notebook", "folder", "笔记", "备忘录", "便签"},
			Tool:        "terminal",
			Action:      "exec",
			Params:      []string{"command", "args"},
			Detail: strings.TrimSpace(`
# Apple Notes CLI

- List all notes: memo notes
- Filter by folder: memo notes -f "Folder Name"
- Search notes: memo notes -s "query"
- Add a note: memo notes -a or memo notes -a "Note Title"
- Edit/delete/move/export: memo notes -e, -d, -m, -ex

macOS only. Requires Apple Notes automation permissions. Interactive prompts require terminal access.
`),
		},
		{
			Name:        "apple-reminders",
			Description: "Manage Apple Reminders via the remindctl CLI on macOS.",
			Terminal:    true,
			Command:     "remindctl",
			Install:     "brew install steipete/tap/remindctl",
			Keywords:    []string{"reminder", "todo", "task", "checklist", "待办", "提醒", "备忘", "事项"},
			Tool:        "terminal",
			Action:      "exec",
			Params:      []string{"command", "args"},
			Detail: strings.TrimSpace(`
# Apple Reminders CLI

- Check permissions: remindctl status, remindctl authorize
- View: remindctl today, tomorrow, week, overdue, upcoming, completed, all
- Lists: remindctl list, remindctl list Work --create
- Add/edit/complete/delete: remindctl add, edit, complete, delete
- Output: --json, --plain, --quiet
`),
		},
		{
			Name:             "evolution-operator",
			Description:      "Trigger and inspect the built-in evolution engine from chat endpoints.",
			Keywords:         []string{"evolution", "coding", "codex", "goal", "自进化", "代码", "需求实现", "状态", "重试"},
			PreferToolResult: true,
			Tool:             "agent.evolution-operator",
			Action:           "execute",
			Params:           []string{"input"},
			DirectCommands:   []string{"/evolve", "/coding", "/codex"},
			Detail: strings.TrimSpace(`
# Evolution Operator

Direct commands include /evolve <goal>, /coding <goal>, /evolve status, /evolve tick,
/codex status, /codex model [model], and /codex effort [minimal|low|medium|high|xhigh].
`),
		},
		{
			Name:             "market-analysis",
			Description:      "A-share/ETF/fund portfolio analysis for midday or close phases.",
			Keywords:         []string{"market", "analysis", "a股", "etf", "基金", "盘中", "收盘", "行情", "趋势", "信号"},
			PreferToolResult: true,
			Tool:             "agent.market-analysis",
			Action:           "execute",
			Params:           []string{"input"},
			DirectCommands:   []string{"/market"},
			Detail: strings.TrimSpace(`
# Market Analysis

Direct commands include /market midday, /market close, /market status, /market portfolio,
and /market add <code> <quantity> <avg_cost> [name].
`),
		},
		{
			Name:             "topic-summary",
			Description:      "Generate daily digest from configurable RSS sources with source/profile commands.",
			Keywords:         []string{"topic summary", "rss", "digest", "ai news", "日报", "新闻摘要"},
			PreferToolResult: true,
			Tool:             "agent.topic-summary",
			Action:           "execute",
			Params:           []string{"input"},
			DirectCommands:   []string{"/topic"},
			Detail: strings.TrimSpace(`
# Topic Summary

Direct commands include /topic run, /topic profile list/add/use, /topic source list/add/update/enable/disable/delete,
/topic config, and /topic state.
`),
		},
		{
			Name:             "writing-organizer",
			Description:      "Organize fragmented writing inputs into topic summaries, outlines, and drafts.",
			Keywords:         []string{"writing", "organizer", "summary", "draft", "写作", "整理", "草稿"},
			PreferToolResult: true,
			Tool:             "agent.writing-organizer",
			Action:           "execute",
			Params:           []string{"input"},
			DirectCommands:   []string{"/writing"},
			Detail: strings.TrimSpace(`
# Writing Organizer

Direct commands include /writing topics, /writing show, /writing append, /writing summarize,
/writing restore, and /writing set.
`),
		},
	}
}

func cloneCapabilities(items []models.AgentCapabilityInfo) []models.AgentCapabilityInfo {
	out := make([]models.AgentCapabilityInfo, 0, len(items))
	for _, item := range items {
		out = append(out, cloneCapability(item))
	}
	return out
}

func cloneCapability(item models.AgentCapabilityInfo) models.AgentCapabilityInfo {
	item.Keywords = append([]string{}, item.Keywords...)
	item.DirectCommands = append([]string{}, item.DirectCommands...)
	item.Params = append([]string{}, item.Params...)
	return item
}
