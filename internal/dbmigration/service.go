package dbmigration

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	appdb "github.com/yourorg/agent_customer_service/internal/db"
)

const DefaultBatchSize = 500

var ErrTargetNotEmpty = errors.New("target database contains business data")

type Options struct {
	BatchSize     int
	AllowExisting bool
}

type TableResult struct {
	Table      string
	SourceRows int64
	TargetRows int64
	CopiedRows int64
	Skipped    bool
}

type Result struct {
	Tables []TableResult
}

func (r Result) CopiedRows() int64 {
	var total int64
	for _, table := range r.Tables {
		total += table.CopiedRows
	}
	return total
}

type tableMigrator func(context.Context, *gorm.DB, *gorm.DB, int) (TableResult, error)

// Service copies the application's business tables from one GORM database to
// another. It is intentionally independent of the concrete destination driver
// so it can be tested without requiring a running MySQL instance.
type Service struct {
	source *gorm.DB
	target *gorm.DB
	tables []tableMigrator
}

func New(source, target *gorm.DB) *Service {
	return &Service{
		source: source,
		target: target,
		tables: []tableMigrator{
			migrateTable[appdb.AdminUser],
			migrateTable[appdb.CallerToken],
			migrateTable[appdb.QAItem],
			migrateTable[appdb.UnansweredQuestion],
			migrateTable[appdb.UnansweredQuestionEvent],
			migrateTable[appdb.AIProvider],
			migrateTable[appdb.AIProviderProfile],
			migrateTable[appdb.Prompt],
			migrateTable[appdb.AgentConfig],
			migrateTable[appdb.ChatLightRule],
			migrateTable[appdb.AIErrorLog],
			migrateTable[appdb.Session],
			migrateTable[appdb.Message],
			migrateTable[appdb.WeComArchiveConfig],
			migrateTable[appdb.WeComMessage],
			migrateTable[appdb.WeComAnalysisRun],
		},
	}
}

func (s *Service) Run(ctx context.Context, opts Options) (Result, error) {
	if s.source == nil || s.target == nil {
		return Result{}, errors.New("source and target databases are required")
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = DefaultBatchSize
	}

	if !opts.AllowExisting {
		populated, err := hasBusinessData(ctx, s.target)
		if err != nil {
			return Result{}, fmt.Errorf("inspect target database: %w", err)
		}
		if populated != "" {
			return Result{}, fmt.Errorf("%w (table %s); use allow-existing only when an upsert merge is intended", ErrTargetNotEmpty, populated)
		}
	}

	if err := appdb.AutoMigrate(s.target.WithContext(ctx)); err != nil {
		return Result{}, fmt.Errorf("create target schema: %w", err)
	}

	var result Result
	err := s.source.WithContext(ctx).Transaction(func(sourceTx *gorm.DB) error {
		return s.target.WithContext(ctx).Transaction(func(targetTx *gorm.DB) error {
			for _, migrate := range s.tables {
				table, err := migrate(ctx, sourceTx, targetTx, opts.BatchSize)
				if err != nil {
					return err
				}
				result.Tables = append(result.Tables, table)
			}
			return nil
		})
	})
	if err != nil {
		return Result{}, fmt.Errorf("copy database: %w", err)
	}
	return result, nil
}

func hasBusinessData(ctx context.Context, target *gorm.DB) (string, error) {
	for _, model := range appdb.Models() {
		if !target.Migrator().HasTable(model) {
			continue
		}
		var count int64
		if err := target.WithContext(ctx).Model(model).Count(&count).Error; err != nil {
			return "", err
		}
		if count > 0 {
			stmt := &gorm.Statement{DB: target}
			if err := stmt.Parse(model); err != nil {
				return "", err
			}
			return stmt.Schema.Table, nil
		}
	}
	return "", nil
}

func migrateTable[T any](ctx context.Context, source, target *gorm.DB, batchSize int) (TableResult, error) {
	var model T
	stmt := &gorm.Statement{DB: source}
	if err := stmt.Parse(&model); err != nil {
		return TableResult{}, fmt.Errorf("resolve model table: %w", err)
	}
	result := TableResult{Table: stmt.Schema.Table}

	if !source.Migrator().HasTable(&model) {
		result.Skipped = true
		return result, nil
	}
	if err := source.WithContext(ctx).Model(&model).Count(&result.SourceRows).Error; err != nil {
		return result, fmt.Errorf("count source table %s: %w", result.Table, err)
	}

	var rows []T
	err := source.WithContext(ctx).FindInBatches(&rows, batchSize, func(_ *gorm.DB, _ int) error {
		if err := target.WithContext(ctx).
			Clauses(clause.OnConflict{UpdateAll: true}).
			Create(&rows).Error; err != nil {
			return fmt.Errorf("write table %s: %w", result.Table, err)
		}
		if err := verifyPrimaryKeys(ctx, target, &model, stmt, rows); err != nil {
			return fmt.Errorf("verify table %s batch: %w", result.Table, err)
		}
		result.CopiedRows += int64(len(rows))
		return nil
	}).Error
	if err != nil {
		return result, fmt.Errorf("read table %s: %w", result.Table, err)
	}
	if result.CopiedRows != result.SourceRows {
		return result, fmt.Errorf("verify table %s: read %d of %d source rows", result.Table, result.CopiedRows, result.SourceRows)
	}
	if err := target.WithContext(ctx).Model(&model).Count(&result.TargetRows).Error; err != nil {
		return result, fmt.Errorf("count target table %s: %w", result.Table, err)
	}
	if result.TargetRows < result.SourceRows {
		return result, fmt.Errorf("verify table %s: target has %d rows, source has %d", result.Table, result.TargetRows, result.SourceRows)
	}
	return result, nil
}

func verifyPrimaryKeys[T any](ctx context.Context, target *gorm.DB, model *T, stmt *gorm.Statement, rows []T) error {
	primaryField := stmt.Schema.PrioritizedPrimaryField
	if primaryField == nil {
		return errors.New("model has no primary key")
	}
	values := make([]any, 0, len(rows))
	for i := range rows {
		value, _ := primaryField.ValueOf(ctx, reflect.ValueOf(&rows[i]))
		values = append(values, value)
	}
	var count int64
	err := target.WithContext(ctx).Model(model).Where(clause.IN{
		Column: clause.Column{Name: primaryField.DBName},
		Values: values,
	}).Count(&count).Error
	if err != nil {
		return err
	}
	if count != int64(len(values)) {
		return fmt.Errorf("found %d of %d primary keys (possible unique-key conflict)", count, len(values))
	}
	return nil
}
