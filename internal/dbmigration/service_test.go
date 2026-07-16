package dbmigration

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	appdb "github.com/gorag-ai/gorag/internal/db"
)

func TestServiceRun_CopiesAndCanUpsertAllModels(t *testing.T) {
	source := openTestDB(t, filepath.Join(t.TempDir(), "source.db"))
	target := openTestDB(t, filepath.Join(t.TempDir(), "target.db"))
	if err := appdb.AutoMigrate(source); err != nil {
		t.Fatalf("migrate source schema: %v", err)
	}
	if err := appdb.AutoMigrate(target); err != nil {
		t.Fatalf("migrate target schema: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	admin := appdb.AdminUser{ID: "admin-1", Username: "admin", PasswordHash: "hash", Role: "admin", CreatedAt: now, UpdatedAt: now}
	session := appdb.Session{ID: "session-1", Title: string([]byte{'o', 'k', 0xff}), Status: "active", Meta: appdb.JSONMap{"channel": "web", "attempt": float64(1)}, CreatedAt: now, UpdatedAt: now}
	message := appdb.Message{ID: "message-1", SessionID: session.ID, Role: "user", Content: "hello", CreatedAt: now}
	caller := appdb.CallerToken{
		ID: "caller-1", Name: "website", TokenHash: "token-hash", Prefix: "tok", Enabled: true,
		AllowedOrigins: appdb.StringSlice{"https://example.com"}, CreatedAt: now, UpdatedAt: now,
	}
	for _, value := range []any{&admin, &caller, &session, &message} {
		if err := source.Create(value).Error; err != nil {
			t.Fatalf("seed %T: %v", value, err)
		}
	}

	service := New(source, target)
	result, err := service.Run(context.Background(), Options{BatchSize: 1})
	if err != nil {
		t.Fatalf("run migration: %v", err)
	}
	if result.CopiedRows() != 4 {
		t.Fatalf("copied rows = %d, want 4", result.CopiedRows())
	}

	var gotSession appdb.Session
	if err := target.First(&gotSession, "id = ?", session.ID).Error; err != nil {
		t.Fatalf("read migrated session: %v", err)
	}
	if gotSession.Meta["channel"] != "web" {
		t.Fatalf("session meta = %#v", gotSession.Meta)
	}
	if gotSession.Title != "ok\uFFFD" {
		t.Fatalf("session title = %q, want repaired UTF-8", gotSession.Title)
	}
	var sanitized int64
	for _, table := range result.Tables {
		sanitized += table.SanitizedStrings
	}
	if sanitized != 1 {
		t.Fatalf("sanitized strings = %d, want 1", sanitized)
	}
	var gotCaller appdb.CallerToken
	if err := target.First(&gotCaller, "id = ?", caller.ID).Error; err != nil {
		t.Fatalf("read migrated caller: %v", err)
	}
	if len(gotCaller.AllowedOrigins) != 1 || gotCaller.AllowedOrigins[0] != "https://example.com" {
		t.Fatalf("allowed origins = %#v", gotCaller.AllowedOrigins)
	}

	if _, err := service.Run(context.Background(), Options{}); !errors.Is(err, ErrTargetNotEmpty) {
		t.Fatalf("second migration error = %v, want ErrTargetNotEmpty", err)
	}
	if err := source.Model(&appdb.AdminUser{}).Where("id = ?", admin.ID).Update("role", "owner").Error; err != nil {
		t.Fatalf("update source admin: %v", err)
	}
	if _, err := service.Run(context.Background(), Options{AllowExisting: true}); err != nil {
		t.Fatalf("upsert migration: %v", err)
	}
	var gotAdmin appdb.AdminUser
	if err := target.First(&gotAdmin, "id = ?", admin.ID).Error; err != nil {
		t.Fatalf("read migrated admin: %v", err)
	}
	if gotAdmin.Role != "owner" {
		t.Fatalf("admin role = %q, want owner", gotAdmin.Role)
	}
}

func TestServiceRun_MissingSourceTablesAreSkipped(t *testing.T) {
	source := openTestDB(t, filepath.Join(t.TempDir(), "source.db"))
	target := openTestDB(t, filepath.Join(t.TempDir(), "target.db"))
	if err := source.AutoMigrate(&appdb.AdminUser{}); err != nil {
		t.Fatalf("migrate source schema: %v", err)
	}

	result, err := New(source, target).Run(context.Background(), Options{})
	if err != nil {
		t.Fatalf("run migration: %v", err)
	}
	var skipped int
	for _, table := range result.Tables {
		if table.Skipped {
			skipped++
		}
	}
	if skipped != len(appdb.Models())-1 {
		t.Fatalf("skipped tables = %d, want %d", skipped, len(appdb.Models())-1)
	}
}

func TestServiceRun_UniqueKeyConflictRollsBackMerge(t *testing.T) {
	source := openTestDB(t, filepath.Join(t.TempDir(), "source.db"))
	target := openTestDB(t, filepath.Join(t.TempDir(), "target.db"))
	if err := appdb.AutoMigrate(source); err != nil {
		t.Fatalf("migrate source schema: %v", err)
	}
	if err := appdb.AutoMigrate(target); err != nil {
		t.Fatalf("migrate target schema: %v", err)
	}
	sourceAdmin := appdb.AdminUser{ID: "source-id", Username: "same-name", PasswordHash: "source-hash", Role: "owner"}
	targetAdmin := appdb.AdminUser{ID: "target-id", Username: "same-name", PasswordHash: "target-hash", Role: "admin"}
	if err := source.Create(&sourceAdmin).Error; err != nil {
		t.Fatalf("seed source: %v", err)
	}
	if err := target.Create(&targetAdmin).Error; err != nil {
		t.Fatalf("seed target: %v", err)
	}

	if _, err := New(source, target).Run(context.Background(), Options{AllowExisting: true}); err == nil {
		t.Fatal("merge succeeded despite a non-primary unique-key conflict")
	}
	var got appdb.AdminUser
	if err := target.First(&got, "id = ?", targetAdmin.ID).Error; err != nil {
		t.Fatalf("read target after rollback: %v", err)
	}
	if got.PasswordHash != targetAdmin.PasswordHash || got.Role != targetAdmin.Role {
		t.Fatalf("target row changed despite rollback: %#v", got)
	}
}

func openTestDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("get test sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return gdb
}
