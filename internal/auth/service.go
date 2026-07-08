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

// --- API Keys ---

func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *Service) CreateAPIKey(ctx context.Context, name, remark string) (plain string, row *db.BizAPIKey, err error) {
	rawBytes := make([]byte, 24)
	if _, err := rand.Read(rawBytes); err != nil {
		return "", nil, err
	}
	plain = "cs_" + hex.EncodeToString(rawBytes)
	row = &db.BizAPIKey{
		ID:        uuid.NewString(),
		Name:      name,
		KeyHash:   HashAPIKey(plain),
		Prefix:    plain[:8],
		Enabled:   true,
		Remark:    remark,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return "", nil, err
	}
	return plain, row, nil
}

func (s *Service) ListAPIKeys(ctx context.Context) ([]db.BizAPIKey, error) {
	var rows []db.BizAPIKey
	err := s.db.WithContext(ctx).Order("created_at desc").Find(&rows).Error
	return rows, err
}

func (s *Service) SetAPIKeyEnabled(ctx context.Context, id string, enabled bool) error {
	return s.db.WithContext(ctx).Model(&db.BizAPIKey{}).Where("id = ?", id).Updates(map[string]any{
		"enabled":    enabled,
		"updated_at": time.Now(),
	}).Error
}

func (s *Service) DeleteAPIKey(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&db.BizAPIKey{}, "id = ?", id).Error
}

func (s *Service) ValidateAPIKey(ctx context.Context, raw string) (*db.BizAPIKey, error) {
	if raw == "" {
		return nil, fmt.Errorf("missing api key")
	}
	var row db.BizAPIKey
	if err := s.db.WithContext(ctx).Where("key_hash = ? AND enabled = ?", HashAPIKey(raw), true).First(&row).Error; err != nil {
		return nil, fmt.Errorf("invalid api key")
	}
	now := time.Now()
	_ = s.db.WithContext(ctx).Model(&row).Update("last_used_at", now).Error
	return &row, nil
}
