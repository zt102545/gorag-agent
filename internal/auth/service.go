package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/yourorg/agent_customer_service/internal/appscope"
	"github.com/yourorg/agent_customer_service/internal/config"
	"github.com/yourorg/agent_customer_service/internal/db"
)

const (
	RoleAdmin  = "admin"
	RoleEditor = "editor"
	RoleViewer = "viewer"
)

type Claims struct {
	UserID   string `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type Service struct {
	db  *gorm.DB
	cfg config.JWTConfig
}

type CallerIdentity struct {
	TokenID         string
	Name            string
	CallerUserID    string
	ChannelID       string
	TenantID        string
	AuthType        string
	IPRatePerMinute int
	IPDailyQuota    int
	PromptGroupID   string
	KnowledgeBaseID string
}

type UserTokenClaims struct {
	AppID           string `json:"app_id"`
	CallerUserID    string `json:"caller_user_id"`
	ChannelID       string `json:"channel_id,omitempty"`
	TenantID        string `json:"tenant_id,omitempty"`
	TokenType       string `json:"token_type"`
	IPRatePerMinute int    `json:"ip_rate_per_minute,omitempty"`
	IPDailyQuota    int    `json:"ip_daily_quota,omitempty"`
	jwt.RegisteredClaims
}

func NewService(gdb *gorm.DB, cfg config.JWTConfig) *Service {
	return &Service{db: gdb, cfg: cfg}
}

func (s *Service) SeedAdmin(ctx context.Context, username, password string) error {
	if username == "" || password == "" {
		return nil
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&db.AdminUser{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user := &db.AdminUser{
		ID:           uuid.NewString(),
		Username:     username,
		PasswordHash: string(hash),
		Role:         RoleAdmin,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	return s.db.WithContext(ctx).Create(user).Error
}

func (s *Service) Login(ctx context.Context, username, password string) (string, *db.AdminUser, error) {
	var user db.AdminUser
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, fmt.Errorf("invalid credentials")
		}
		return "", nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", nil, fmt.Errorf("invalid credentials")
	}
	token, err := s.issueToken(&user)
	if err != nil {
		return "", nil, err
	}
	return token, &user, nil
}

func (s *Service) issueToken(user *db.AdminUser) (string, error) {
	expire := s.cfg.Expire
	if expire <= 0 {
		expire = 24 * time.Hour
	}
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(s.cfg.Secret))
}

func (s *Service) ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.cfg.Secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func (s *Service) GetUser(ctx context.Context, id string) (*db.AdminUser, error) {
	var user db.AdminUser
	if err := s.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Service) ListUsers(ctx context.Context) ([]db.AdminUser, error) {
	var users []db.AdminUser
	err := s.db.WithContext(ctx).Order("created_at asc").Find(&users).Error
	return users, err
}

type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role" binding:"required"`
}

func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest) (*db.AdminUser, error) {
	role := normalizeRole(req.Role)
	if role == "" {
		return nil, fmt.Errorf("invalid role")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &db.AdminUser{
		ID:           uuid.NewString(),
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

type UpdateUserRequest struct {
	Role     string `json:"role"`
	Password string `json:"password"`
}

func (s *Service) UpdateUser(ctx context.Context, id string, req UpdateUserRequest) (*db.AdminUser, error) {
	user, err := s.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Role != "" {
		role := normalizeRole(req.Role)
		if role == "" {
			return nil, fmt.Errorf("invalid role")
		}
		user.Role = role
	}
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		user.PasswordHash = string(hash)
	}
	user.UpdatedAt = time.Now()
	if err := s.db.WithContext(ctx).Save(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) DeleteUser(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&db.AdminUser{}, "id = ?", id).Error
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case RoleAdmin, RoleEditor, RoleViewer:
		return strings.ToLower(strings.TrimSpace(role))
	default:
		return ""
	}
}

func RoleAtLeast(role, min string) bool {
	rank := map[string]int{RoleViewer: 1, RoleEditor: 2, RoleAdmin: 3}
	return rank[role] >= rank[min]
}

func HashSecret(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func generateSecret(prefix string, size int) (string, error) {
	rawBytes := make([]byte, size)
	if _, err := rand.Read(rawBytes); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(rawBytes), nil
}

type CreateCallerTokenRequest struct {
	Name            string     `json:"name" binding:"required"`
	ChannelID       string     `json:"channel_id"`
	TenantID        string     `json:"tenant_id"`
	Remark          string     `json:"remark"`
	ExpiresAt       *time.Time `json:"expires_at"`
	PublicAccess    bool       `json:"public_access"`
	AllowedOrigins  []string   `json:"allowed_origins"`
	IPRatePerMinute int        `json:"ip_rate_per_minute"`
	IPDailyQuota    int        `json:"ip_daily_quota"`
	PromptGroupID   string     `json:"prompt_group_id"`
	KnowledgeBaseID string     `json:"knowledge_base_id"`
}

func (s *Service) CreateCallerToken(ctx context.Context, req CreateCallerTokenRequest) (plain string, row *db.CallerToken, err error) {
	hasExplicitScope := strings.TrimSpace(req.PromptGroupID) != "" || strings.TrimSpace(req.KnowledgeBaseID) != ""
	req.PromptGroupID = firstNonEmpty(req.PromptGroupID, appscope.DefaultPromptGroupID)
	req.KnowledgeBaseID = firstNonEmpty(req.KnowledgeBaseID, appscope.DefaultKnowledgeBaseID)
	if hasExplicitScope {
		if err := s.validateScopeBindings(ctx, req.PromptGroupID, req.KnowledgeBaseID); err != nil {
			return "", nil, err
		}
	}
	channelID := strings.TrimSpace(req.ChannelID)
	if req.PublicAccess && channelID == "" {
		return "", nil, fmt.Errorf("channel_id is required for public access")
	}
	if req.PublicAccess {
		if err := s.ensurePublicChannelAvailable(ctx, channelID, ""); err != nil {
			return "", nil, err
		}
	}
	if req.IPRatePerMinute < 0 || req.IPDailyQuota < 0 {
		return "", nil, fmt.Errorf("public widget limits cannot be negative")
	}
	ratePerMinute, dailyQuota := publicLimitDefaults(req.IPRatePerMinute, req.IPDailyQuota)
	plain, err = generateSecret("as_", 24)
	if err != nil {
		return "", nil, err
	}
	row = &db.CallerToken{
		ID:              uuid.NewString(),
		Name:            strings.TrimSpace(req.Name),
		TokenHash:       HashSecret(plain), // legacy compatibility
		PlainToken:      "",
		Prefix:          plain[:10],
		AppID:           "app_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		AppSecretHash:   HashSecret(plain),
		ChannelID:       channelID,
		TenantID:        strings.TrimSpace(req.TenantID),
		Enabled:         true,
		Remark:          strings.TrimSpace(req.Remark),
		ExpiresAt:       req.ExpiresAt,
		PublicAccess:    req.PublicAccess,
		AllowedOrigins:  normalizeOrigins(req.AllowedOrigins),
		IPRatePerMinute: ratePerMinute,
		IPDailyQuota:    dailyQuota,
		PromptGroupID:   req.PromptGroupID,
		KnowledgeBaseID: req.KnowledgeBaseID,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return "", nil, err
	}
	return plain, row, nil
}

type IssueUserTokenRequest struct {
	AppID        string `json:"app_id" binding:"required"`
	AppSecret    string `json:"app_secret" binding:"required"`
	CallerUserID string `json:"caller_user_id" binding:"required"`
	ChannelID    string `json:"channel_id"`
	TenantID     string `json:"tenant_id"`
}

type IssuePublicUserTokenRequest struct {
	CallerUserID string `json:"caller_user_id" binding:"required"`
	ChannelID    string `json:"channel_id" binding:"required"`
}

type PublicTokenPolicy struct {
	ChannelID       string
	IPRatePerMinute int
	IPDailyQuota    int
}

func (s *Service) IssuePublicUserToken(ctx context.Context, req IssuePublicUserTokenRequest, origin string) (string, time.Time, PublicTokenPolicy, error) {
	callerUserID := strings.TrimSpace(req.CallerUserID)
	channelID := strings.TrimSpace(req.ChannelID)
	if callerUserID == "" || channelID == "" {
		return "", time.Time{}, PublicTokenPolicy{}, fmt.Errorf("caller_user_id and channel_id are required")
	}

	var apps []db.CallerToken
	if err := s.db.WithContext(ctx).Where("channel_id = ? AND public_access = ? AND enabled = ?", channelID, true, true).Limit(2).Find(&apps).Error; err != nil {
		return "", time.Time{}, PublicTokenPolicy{}, err
	}
	if len(apps) != 1 {
		return "", time.Time{}, PublicTokenPolicy{}, fmt.Errorf("public channel is unavailable")
	}
	app := apps[0]
	if app.ExpiresAt != nil && app.ExpiresAt.Before(time.Now()) {
		return "", time.Time{}, PublicTokenPolicy{}, fmt.Errorf("public channel is unavailable")
	}
	if !originAllowed(app.AllowedOrigins, origin) {
		return "", time.Time{}, PublicTokenPolicy{}, fmt.Errorf("origin is not allowed for this channel")
	}

	ratePerMinute, dailyQuota := publicLimitDefaults(app.IPRatePerMinute, app.IPDailyQuota)
	now := time.Now()
	expiresAt := now.Add(15 * time.Minute)
	claims := UserTokenClaims{
		AppID: app.AppID, CallerUserID: callerUserID, ChannelID: app.ChannelID, TenantID: app.TenantID,
		TokenType: "public_widget", IPRatePerMinute: ratePerMinute, IPDailyQuota: dailyQuota,
		RegisteredClaims: jwt.RegisteredClaims{Issuer: "agent-customer-service", Subject: callerUserID, Audience: jwt.ClaimStrings{"biz-api"}, IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)), ExpiresAt: jwt.NewNumericDate(expiresAt), ID: uuid.NewString()},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return "", time.Time{}, PublicTokenPolicy{}, err
	}
	_ = s.db.WithContext(ctx).Model(&app).Update("last_used_at", now).Error
	return token, expiresAt, PublicTokenPolicy{ChannelID: app.ChannelID, IPRatePerMinute: ratePerMinute, IPDailyQuota: dailyQuota}, nil
}

func (s *Service) IssueUserToken(ctx context.Context, req IssueUserTokenRequest) (string, time.Time, error) {
	var app db.CallerToken
	if err := s.db.WithContext(ctx).Where("app_id = ? AND enabled = ?", strings.TrimSpace(req.AppID), true).First(&app).Error; err != nil {
		return "", time.Time{}, fmt.Errorf("invalid app credentials")
	}
	if app.AppSecretHash == "" || app.AppSecretHash != HashSecret(strings.TrimSpace(req.AppSecret)) {
		return "", time.Time{}, fmt.Errorf("invalid app credentials")
	}
	if app.ExpiresAt != nil && app.ExpiresAt.Before(time.Now()) {
		return "", time.Time{}, fmt.Errorf("app credentials expired")
	}
	callerUserID := strings.TrimSpace(req.CallerUserID)
	channelID := strings.TrimSpace(req.ChannelID)
	tenantID := strings.TrimSpace(req.TenantID)
	if callerUserID == "" {
		return "", time.Time{}, fmt.Errorf("caller_user_id is required")
	}
	if app.ChannelID != "" {
		if channelID != "" && channelID != app.ChannelID {
			return "", time.Time{}, fmt.Errorf("channel_id is not allowed for this app")
		}
		channelID = app.ChannelID
	}
	if app.TenantID != "" {
		if tenantID != "" && tenantID != app.TenantID {
			return "", time.Time{}, fmt.Errorf("tenant_id is not allowed for this app")
		}
		tenantID = app.TenantID
	}
	now := time.Now()
	expiresAt := now.Add(15 * time.Minute)
	claims := UserTokenClaims{
		AppID: app.AppID, CallerUserID: callerUserID, ChannelID: channelID, TenantID: tenantID, TokenType: "biz_user",
		RegisteredClaims: jwt.RegisteredClaims{Issuer: "agent-customer-service", Subject: callerUserID, Audience: jwt.ClaimStrings{"biz-api"}, IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)), ExpiresAt: jwt.NewNumericDate(expiresAt), ID: uuid.NewString()},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return "", time.Time{}, err
	}
	_ = s.db.WithContext(ctx).Model(&app).Update("last_used_at", now).Error
	return token, expiresAt, nil
}

func (s *Service) validateUserToken(raw string) (*CallerIdentity, error) {
	token, err := jwt.ParseWithClaims(raw, &UserTokenClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.cfg.Secret), nil
	}, jwt.WithAudience("biz-api"), jwt.WithIssuer("agent-customer-service"))
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*UserTokenClaims)
	if !ok || !token.Valid || (claims.TokenType != "biz_user" && claims.TokenType != "public_widget") || claims.CallerUserID == "" || claims.AppID == "" {
		return nil, fmt.Errorf("invalid user token")
	}
	var app db.CallerToken
	if err := s.db.Where("app_id = ? AND enabled = ?", claims.AppID, true).First(&app).Error; err != nil {
		return nil, fmt.Errorf("app disabled or deleted")
	}
	if app.ExpiresAt != nil && app.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("app credentials expired")
	}
	authType := "user_token"
	ratePerMinute := claims.IPRatePerMinute
	dailyQuota := claims.IPDailyQuota
	if claims.TokenType == "public_widget" {
		if !app.PublicAccess {
			return nil, fmt.Errorf("public channel disabled")
		}
		if claims.ChannelID != app.ChannelID || claims.TenantID != app.TenantID {
			return nil, fmt.Errorf("public channel scope changed")
		}
		ratePerMinute, dailyQuota = publicLimitDefaults(app.IPRatePerMinute, app.IPDailyQuota)
		authType = "public_user_token"
	}
	return &CallerIdentity{TokenID: app.ID, Name: app.Name, CallerUserID: claims.CallerUserID, ChannelID: claims.ChannelID, TenantID: claims.TenantID, AuthType: authType, IPRatePerMinute: ratePerMinute, IPDailyQuota: dailyQuota, PromptGroupID: firstNonEmpty(app.PromptGroupID, appscope.DefaultPromptGroupID), KnowledgeBaseID: firstNonEmpty(app.KnowledgeBaseID, appscope.DefaultKnowledgeBaseID)}, nil
}

func (s *Service) ListCallerTokens(ctx context.Context) ([]db.CallerToken, error) {
	var rows []db.CallerToken
	err := s.db.WithContext(ctx).Order("created_at desc").Find(&rows).Error
	return rows, err
}

type UpdateCallerTokenRequest struct {
	Enabled         *bool      `json:"enabled"`
	Name            *string    `json:"name"`
	ChannelID       *string    `json:"channel_id"`
	TenantID        *string    `json:"tenant_id"`
	Remark          *string    `json:"remark"`
	ExpiresAt       *time.Time `json:"expires_at"`
	PublicAccess    *bool      `json:"public_access"`
	AllowedOrigins  *[]string  `json:"allowed_origins"`
	IPRatePerMinute *int       `json:"ip_rate_per_minute"`
	IPDailyQuota    *int       `json:"ip_daily_quota"`
	PromptGroupID   *string    `json:"prompt_group_id"`
	KnowledgeBaseID *string    `json:"knowledge_base_id"`
}

func (s *Service) UpdateCallerToken(ctx context.Context, id string, req UpdateCallerTokenRequest) error {
	var current db.CallerToken
	if err := s.db.WithContext(ctx).First(&current, "id = ?", id).Error; err != nil {
		return err
	}
	channelID := current.ChannelID
	if req.ChannelID != nil {
		channelID = strings.TrimSpace(*req.ChannelID)
	}
	publicAccess := current.PublicAccess
	if req.PublicAccess != nil {
		publicAccess = *req.PublicAccess
	}
	if publicAccess && channelID == "" {
		return fmt.Errorf("channel_id is required for public access")
	}
	if publicAccess {
		if err := s.ensurePublicChannelAvailable(ctx, channelID, id); err != nil {
			return err
		}
	}
	updates := map[string]any{
		"updated_at": time.Now(),
	}
	promptGroupID, knowledgeBaseID := firstNonEmpty(current.PromptGroupID, appscope.DefaultPromptGroupID), firstNonEmpty(current.KnowledgeBaseID, appscope.DefaultKnowledgeBaseID)
	if req.PromptGroupID != nil {
		promptGroupID = firstNonEmpty(*req.PromptGroupID, appscope.DefaultPromptGroupID)
		updates["prompt_group_id"] = promptGroupID
	}
	if req.KnowledgeBaseID != nil {
		knowledgeBaseID = firstNonEmpty(*req.KnowledgeBaseID, appscope.DefaultKnowledgeBaseID)
		updates["knowledge_base_id"] = knowledgeBaseID
	}
	if req.PromptGroupID != nil || req.KnowledgeBaseID != nil {
		if err := s.validateScopeBindings(ctx, promptGroupID, knowledgeBaseID); err != nil {
			return err
		}
	}
	if publicAccess && current.AppID == "" {
		updates["app_id"] = "app_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Name != nil {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.ChannelID != nil {
		updates["channel_id"] = strings.TrimSpace(*req.ChannelID)
	}
	if req.TenantID != nil {
		updates["tenant_id"] = strings.TrimSpace(*req.TenantID)
	}
	if req.Remark != nil {
		updates["remark"] = strings.TrimSpace(*req.Remark)
	}
	if req.ExpiresAt != nil {
		updates["expires_at"] = *req.ExpiresAt
	}
	if req.PublicAccess != nil {
		updates["public_access"] = *req.PublicAccess
	}
	if req.AllowedOrigins != nil {
		updates["allowed_origins"] = normalizeOrigins(*req.AllowedOrigins)
	}
	if req.IPRatePerMinute != nil {
		if *req.IPRatePerMinute <= 0 {
			return fmt.Errorf("ip_rate_per_minute must be greater than 0")
		}
		updates["ip_rate_per_minute"] = *req.IPRatePerMinute
	}
	if req.IPDailyQuota != nil {
		if *req.IPDailyQuota <= 0 {
			return fmt.Errorf("ip_daily_quota must be greater than 0")
		}
		updates["ip_daily_quota"] = *req.IPDailyQuota
	}
	return s.db.WithContext(ctx).Model(&db.CallerToken{}).Where("id = ?", id).Updates(updates).Error
}

func (s *Service) ensurePublicChannelAvailable(ctx context.Context, channelID, excludeID string) error {
	query := s.db.WithContext(ctx).Model(&db.CallerToken{}).Where("channel_id = ? AND public_access = ?", channelID, true)
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("public channel_id already exists")
	}
	return nil
}

func publicLimitDefaults(ratePerMinute, dailyQuota int) (int, int) {
	if ratePerMinute <= 0 {
		ratePerMinute = 60
	}
	if dailyQuota <= 0 {
		dailyQuota = 2000
	}
	return ratePerMinute, dailyQuota
}

func normalizeOrigins(values []string) db.StringSlice {
	seen := map[string]struct{}{}
	result := make(db.StringSlice, 0, len(values))
	for _, value := range values {
		value = strings.TrimRight(strings.TrimSpace(value), "/")
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func originAllowed(allowed []string, origin string) bool {
	if len(allowed) == 0 {
		return true
	}
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	for _, value := range allowed {
		if value == "*" || strings.EqualFold(strings.TrimRight(strings.TrimSpace(value), "/"), origin) {
			return true
		}
	}
	return false
}

func (s *Service) DeleteCallerToken(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&db.CallerToken{}, "id = ?", id).Error
}

func (s *Service) ValidateCallerToken(ctx context.Context, raw string) (*CallerIdentity, error) {
	if raw == "" {
		return nil, fmt.Errorf("missing caller token")
	}
	if identity, err := s.validateUserToken(raw); err == nil {
		return identity, nil
	}
	var row db.CallerToken
	if err := s.db.WithContext(ctx).Where("token_hash = ? AND enabled = ?", HashSecret(raw), true).First(&row).Error; err != nil {
		return nil, fmt.Errorf("invalid caller token")
	}
	if row.ExpiresAt != nil && row.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("caller token expired")
	}
	now := time.Now()
	_ = s.db.WithContext(ctx).Model(&row).Update("last_used_at", now).Error
	return &CallerIdentity{
		TokenID:         row.ID,
		Name:            row.Name,
		ChannelID:       row.ChannelID,
		TenantID:        row.TenantID,
		AuthType:        "legacy_caller_token",
		PromptGroupID:   firstNonEmpty(row.PromptGroupID, appscope.DefaultPromptGroupID),
		KnowledgeBaseID: firstNonEmpty(row.KnowledgeBaseID, appscope.DefaultKnowledgeBaseID),
	}, nil
}

func (s *Service) validateScopeBindings(ctx context.Context, promptGroupID, knowledgeBaseID string) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&db.PromptGroup{}).Where("id = ?", promptGroupID).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("prompt group not found")
	}
	if err := s.db.WithContext(ctx).Model(&db.KnowledgeBase{}).Where("id = ?", knowledgeBaseID).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("knowledge base not found")
	}
	return nil
}

func (s *Service) CanAccessSession(ctx context.Context, sessionID, tokenID, callerUserID, channelID, tenantID string) bool {
	var row db.Session
	if err := s.db.WithContext(ctx).Select("caller_token_id", "external_user_id", "channel_id", "tenant_id").First(&row, "id = ?", sessionID).Error; err != nil {
		return false
	}
	return row.CallerTokenID == tokenID && row.ExternalUserID == callerUserID && row.ChannelID == channelID && row.TenantID == tenantID
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
