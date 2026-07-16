package admin

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gorag-ai/gorag/internal/db"
)

const (
	ChatRuleMatchExact    = "exact"
	ChatRuleMatchContains = "contains"
	ChatRuleMatchRegex    = "regex"
	ChatRuleMatchKeywords = "keywords"

	ChatRuleActionReply          = "reply"
	ChatRuleActionHandoff        = "handoff"
	ChatRuleActionReject         = "reject"
	ChatRuleActionContinueSearch = "continue_search"
)

type ChatLightRuleView struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Enabled      bool      `json:"enabled"`
	Priority     int       `json:"priority"`
	MatchType    string    `json:"match_type"`
	Pattern      string    `json:"pattern"`
	ActionType   string    `json:"action_type"`
	ResponseText string    `json:"response_text,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type UpsertChatLightRuleRequest struct {
	Name         string `json:"name"`
	Enabled      *bool  `json:"enabled"`
	Priority     *int   `json:"priority"`
	MatchType    string `json:"match_type"`
	Pattern      string `json:"pattern"`
	ActionType   string `json:"action_type"`
	ResponseText string `json:"response_text"`
}

func (s *SettingsService) ListChatLightRules(ctx context.Context, enabledOnly bool) ([]ChatLightRuleView, error) {
	q := s.db.WithContext(ctx).Model(&db.ChatLightRule{})
	if enabledOnly {
		q = q.Where("enabled = ?", true)
	}
	var rows []db.ChatLightRule
	if err := q.Order("priority desc, updated_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ChatLightRuleView, 0, len(rows))
	for _, row := range rows {
		out = append(out, toChatLightRuleView(row))
	}
	return out, nil
}

func (s *SettingsService) CreateChatLightRule(ctx context.Context, req UpsertChatLightRuleRequest) (*ChatLightRuleView, error) {
	if err := validateChatLightRule(req); err != nil {
		return nil, err
	}
	now := time.Now()
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	priority := 100
	if req.Priority != nil {
		priority = *req.Priority
	}
	row := &db.ChatLightRule{
		ID:           uuid.NewString(),
		Name:         strings.TrimSpace(req.Name),
		Enabled:      enabled,
		Priority:     priority,
		MatchType:    normalizeChatRuleMatchType(req.MatchType),
		Pattern:      strings.TrimSpace(req.Pattern),
		ActionType:   normalizeChatRuleActionType(req.ActionType),
		ResponseText: strings.TrimSpace(req.ResponseText),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, err
	}
	view := toChatLightRuleView(*row)
	return &view, nil
}

func (s *SettingsService) UpdateChatLightRule(ctx context.Context, id string, req UpsertChatLightRuleRequest) (*ChatLightRuleView, error) {
	if err := validateChatLightRule(req); err != nil {
		return nil, err
	}
	var row db.ChatLightRule
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	row.Name = strings.TrimSpace(req.Name)
	if req.Enabled != nil {
		row.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		row.Priority = *req.Priority
	}
	row.MatchType = normalizeChatRuleMatchType(req.MatchType)
	row.Pattern = strings.TrimSpace(req.Pattern)
	row.ActionType = normalizeChatRuleActionType(req.ActionType)
	row.ResponseText = strings.TrimSpace(req.ResponseText)
	row.UpdatedAt = time.Now()
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return nil, err
	}
	view := toChatLightRuleView(row)
	return &view, nil
}

func (s *SettingsService) DeleteChatLightRule(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&db.ChatLightRule{}, "id = ?", id).Error
}

func validateChatLightRule(req UpsertChatLightRuleRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("name is required")
	}
	matchType := normalizeChatRuleMatchType(req.MatchType)
	switch matchType {
	case ChatRuleMatchExact, ChatRuleMatchContains, ChatRuleMatchRegex, ChatRuleMatchKeywords:
	default:
		return fmt.Errorf("unsupported match_type: %s", req.MatchType)
	}
	pattern := strings.TrimSpace(req.Pattern)
	if pattern == "" {
		return fmt.Errorf("pattern is required")
	}
	if matchType == ChatRuleMatchRegex {
		if len(pattern) > 256 {
			return fmt.Errorf("regex pattern is too long")
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
	}
	actionType := normalizeChatRuleActionType(req.ActionType)
	switch actionType {
	case ChatRuleActionReply, ChatRuleActionHandoff, ChatRuleActionReject, ChatRuleActionContinueSearch:
	default:
		return fmt.Errorf("unsupported action_type: %s", req.ActionType)
	}
	if actionType == ChatRuleActionReply && strings.TrimSpace(req.ResponseText) == "" {
		return fmt.Errorf("response_text is required for reply action")
	}
	if req.Priority != nil && *req.Priority < 0 {
		return fmt.Errorf("priority must be greater than or equal to 0")
	}
	return nil
}

func normalizeChatRuleMatchType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ChatRuleMatchExact, ChatRuleMatchContains, ChatRuleMatchRegex, ChatRuleMatchKeywords:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeChatRuleActionType(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func toChatLightRuleView(row db.ChatLightRule) ChatLightRuleView {
	return ChatLightRuleView{
		ID:           row.ID,
		Name:         row.Name,
		Enabled:      row.Enabled,
		Priority:     row.Priority,
		MatchType:    row.MatchType,
		Pattern:      row.Pattern,
		ActionType:   row.ActionType,
		ResponseText: row.ResponseText,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}
