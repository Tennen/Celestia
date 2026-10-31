package accounts

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type RosterEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type MenuState struct {
	Initialized bool          `json:"initialized"`
	Accounts    []RosterEntry `json:"accounts"`
	CheckedAt   time.Time     `json:"checked_at"`
	PublishedAt *time.Time    `json:"published_at,omitempty"`
	Error       string        `json:"error,omitempty"`
}

func canonicalRoster(list []Account) ([]RosterEntry, error) {
	roster := make([]RosterEntry, 0, len(list))
	seen := map[string]bool{}
	for _, a := range list {
		if a.ID == "" || strings.TrimSpace(a.Name) == "" || seen[a.ID] {
			return nil, errors.New("花园账号列表存在空值或重复 ID")
		}
		seen[a.ID] = true
		roster = append(roster, RosterEntry{ID: a.ID, Name: a.Name})
	}
	sort.Slice(roster, func(i, j int) bool { return roster[i].ID < roster[j].ID })
	return roster, nil
}

func (s *Service) Sync(ctx context.Context) (published bool, err error) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	var state MenuState
	if err = s.load(ctx, MenuStateKey, &state); err != nil {
		return false, err
	}
	state.CheckedAt = time.Now().UTC()
	state.Error = ""
	defer func() {
		if err != nil {
			state.Error = err.Error()
		}
		if saveErr := s.save(ctx, MenuStateKey, state); saveErr != nil {
			err = errors.Join(err, saveErr)
		}
	}()
	root, err := s.root(ctx)
	if err != nil {
		return false, err
	}
	list, err := s.api.Accounts(ctx, root)
	if err != nil {
		return false, err
	}
	roster, err := canonicalRoster(list)
	if err != nil {
		return false, err
	}
	if state.Initialized && reflect.DeepEqual(state.Accounts, roster) {
		return false, nil
	}
	menu, err := s.menus.GetWeComMenu(ctx)
	if err != nil {
		return false, err
	}
	planned, err := accountMenu(menu.Config, list)
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(menuShape(menu.Config.Buttons), menuShape(planned.Buttons)) {
		if _, err = s.menus.SaveWeComMenu(ctx, planned); err != nil {
			return false, err
		}
		now := time.Now().UTC()
		state.PublishedAt = &now
		published = true
	}
	state.Accounts = roster
	state.Initialized = true
	return published, nil
}

func managedGroup(button models.AgentWeComButton) bool {
	if IsMenuKey(button.Key) {
		return true
	}
	if len(button.SubButtons) == 0 {
		return false
	}
	for _, child := range button.SubButtons {
		if !IsMenuKey(child.Key) {
			return false
		}
	}
	return true
}

func accountMenu(current models.AgentWeComMenuConfig, list []Account) (models.AgentWeComMenuConfig, error) {
	buttons := make([]models.AgentWeComButton, 0, 3)
	for _, button := range current.Buttons {
		if !managedGroup(button) {
			buttons = append(buttons, button)
		}
	}
	groups := (len(list) + 4) / 5
	if len(buttons)+groups > 3 {
		return current, fmt.Errorf("企微最多 3 个父菜单、每组 5 个账号；当前 %d 个账号放不下，已保留原菜单", len(list))
	}
	for g := 0; g < groups; g++ {
		name := "花园开关"
		if groups > 1 {
			name = fmt.Sprintf("账号开关%d", g+1)
		}
		button := models.AgentWeComButton{ID: fmt.Sprintf("garden-accounts-%d", g+1), Name: name, Enabled: true}
		for i := g * 5; i < len(list) && i < (g+1)*5; i++ {
			a := list[i]
			if len(a.Name) > 40 {
				return current, fmt.Errorf("账号名称超过企微 40 字节限制：%s", a.Name)
			}
			button.SubButtons = append(button.SubButtons, models.AgentWeComButton{ID: "garden-account-" + a.ID, Name: a.Name, Key: TogglePrefix + a.ID, Enabled: true})
		}
		buttons = append(buttons, button)
	}
	current.Buttons = buttons
	current.Version = 1
	return current, nil
}

type buttonShape struct {
	Name, Key string
	Enabled   bool
	Children  []buttonShape
}

func menuShape(buttons []models.AgentWeComButton) []buttonShape {
	out := make([]buttonShape, 0, len(buttons))
	for _, b := range buttons {
		out = append(out, buttonShape{Name: b.Name, Key: b.Key, Enabled: b.Enabled, Children: menuShape(b.SubButtons)})
	}
	return out
}
