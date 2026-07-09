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
	TokenID   string
	Name      string
	ChannelID string
	TenantID  string
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
	Name      string     `json:"name" binding:"required"`
	ChannelID string     `json:"channel_id"`
	TenantID  string     `json:"tenant_id"`
	Remark    string     `json:"remark"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func (s *Service) CreateCallerToken(ctx context.Context, req CreateCallerTokenRequest) (plain string, row *db.CallerToken, err error) {
	plain, err = generateSecret("ct_", 24)
	if err != nil {
		return "", nil, err
	}
	row = &db.CallerToken{
		ID:        uuid.NewString(),
		Name:      strings.TrimSpace(req.Name),
		TokenHash: HashSecret(plain),
		Prefix:    plain[:10],
		ChannelID: strings.TrimSpace(req.ChannelID),
		TenantID:  strings.TrimSpace(req.TenantID),
		Enabled:   true,
		Remark:    strings.TrimSpace(req.Remark),
		ExpiresAt: req.ExpiresAt,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return "", nil, err
	}
	return plain, row, nil
}

func (s *Service) ListCallerTokens(ctx context.Context) ([]db.CallerToken, error) {
	var rows []db.CallerToken
	err := s.db.WithContext(ctx).Order("created_at desc").Find(&rows).Error
	return rows, err
}

type UpdateCallerTokenRequest struct {
	Enabled   *bool      `json:"enabled"`
	Name      *string    `json:"name"`
	ChannelID *string    `json:"channel_id"`
	TenantID  *string    `json:"tenant_id"`
	Remark    *string    `json:"remark"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func (s *Service) UpdateCallerToken(ctx context.Context, id string, req UpdateCallerTokenRequest) error {
	updates := map[string]any{
		"updated_at": time.Now(),
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
	return s.db.WithContext(ctx).Model(&db.CallerToken{}).Where("id = ?", id).Updates(updates).Error
}

func (s *Service) DeleteCallerToken(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&db.CallerToken{}, "id = ?", id).Error
}

func (s *Service) ValidateCallerToken(ctx context.Context, raw string) (*CallerIdentity, error) {
	if raw == "" {
		return nil, fmt.Errorf("missing caller token")
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
		TokenID:   row.ID,
		Name:      row.Name,
		ChannelID: row.ChannelID,
		TenantID:  row.TenantID,
	}, nil
}
