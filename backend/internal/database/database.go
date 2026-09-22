package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/blueship581/museum-conservation-approval-control/backend/internal/config"
	"github.com/blueship581/museum-conservation-approval-control/backend/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(ctx context.Context, cfg config.Config, log *slog.Logger) (*gorm.DB, *redis.Client, error) {
	var dialector gorm.Dialector
	switch cfg.DatabaseDriver {
	case "postgres":
		dialector = postgres.Open(cfg.DatabaseDSN)
	case "mysql":
		dialector = mysql.Open(cfg.DatabaseDSN)
	case "sqlite":
		dialector = sqlite.Open(cfg.DatabaseDSN)
	default:
		return nil, nil, fmt.Errorf("unsupported database driver %q", cfg.DatabaseDriver)
	}
	logLevel := logger.Warn
	if cfg.Environment == "development" {
		logLevel = logger.Info
	}
	var db *gorm.DB
	var err error
	for attempt := 1; attempt <= 20; attempt++ {
		db, err = gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logLevel)})
		if err == nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil && sqlDB.PingContext(ctx) == nil {
				break
			}
			if dbErr != nil {
				err = dbErr
			} else {
				err = sqlDB.PingContext(ctx)
			}
		}
		log.Warn("database not ready", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if err != nil {
		return nil, nil, fmt.Errorf("connect database: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, nil, err
	}
	if err := Seed(ctx, db); err != nil {
		return nil, nil, err
	}
	var redisClient *redis.Client
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return nil, nil, fmt.Errorf("connect redis: %w", err)
		}
	}
	return db, redisClient, nil
}

func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{}, &model.AuditLog{},
		&model.Artifact{},
		&model.TreatmentPlan{},
		&model.MaterialTest{},
		&model.StageApproval{},
		&model.ApprovalOpinion{},
	)
}

func Seed(ctx context.Context, db *gorm.DB) error {
	var users int64
	if err := db.WithContext(ctx).Model(&model.User{}).Count(&users).Error; err != nil {
		return err
	}
	if users == 0 {
		password, err := bcrypt.GenerateFromPassword([]byte("Admin123!"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		seedUsers := []model.User{
			{Username: "admin", DisplayName: "系统管理员", PasswordHash: string(password), Role: model.RoleAdmin, Active: true},
			{Username: "reviewer", DisplayName: "质量复核员", PasswordHash: string(password), Role: model.RoleReviewer, Active: true},
			{Username: "operator", DisplayName: "现场操作员", PasswordHash: string(password), Role: model.RoleOperator, Active: true},
			{Username: "viewer", DisplayName: "只读观察员", PasswordHash: string(password), Role: model.RoleViewer, Active: true},
		}
		if err := db.WithContext(ctx).Create(&seedUsers).Error; err != nil {
			return err
		}
	}

	if err := seedArtifact(ctx, db); err != nil {
		return err
	}

	if err := seedTreatmentPlan(ctx, db); err != nil {
		return err
	}

	if err := seedMaterialTest(ctx, db); err != nil {
		return err
	}

	if err := seedStageApproval(ctx, db); err != nil {
		return err
	}

	return nil
}

func seedArtifact(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.Artifact{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.Artifact{

		{BaseModel: model.BaseModel{Code: "A-001", Name: "文物示例一", Status: "registered", Version: 1,
			Description: "用于启动验证和主要流程演示的文物记录"}, Facility: "文物保护处理审批区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-512-01"},

		{BaseModel: model.BaseModel{Code: "A-002", Name: "文物示例二", Status: "stable", Version: 1,
			Description: "用于启动验证和主要流程演示的文物记录"}, Facility: "文物保护处理审批区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-512-02"},

		{BaseModel: model.BaseModel{Code: "A-003", Name: "文物示例三", Status: "treatment", Version: 1,
			Description: "用于启动验证和主要流程演示的文物记录"}, Facility: "文物保护处理审批区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-512-03"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedTreatmentPlan(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.TreatmentPlan{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.TreatmentPlan{

		{BaseModel: model.BaseModel{Code: "TP-001", Name: "处理方案示例一", Status: "draft", Version: 1,
			Description: "用于启动验证和主要流程演示的处理方案记录"}, Facility: "文物保护处理审批区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-512-01"},

		{BaseModel: model.BaseModel{Code: "TP-002", Name: "处理方案示例二", Status: "review", Version: 1,
			Description: "用于启动验证和主要流程演示的处理方案记录"}, Facility: "文物保护处理审批区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-512-02"},

		{BaseModel: model.BaseModel{Code: "TP-003", Name: "处理方案示例三", Status: "approved", Version: 1,
			Description: "用于启动验证和主要流程演示的处理方案记录"}, Facility: "文物保护处理审批区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-512-03"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedMaterialTest(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.MaterialTest{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.MaterialTest{

		{BaseModel: model.BaseModel{Code: "MT-001", Name: "材料检测示例一", Status: "planned", Version: 1,
			Description: "用于启动验证和主要流程演示的材料检测记录"}, Facility: "文物保护处理审批区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-512-01"},

		{BaseModel: model.BaseModel{Code: "MT-002", Name: "材料检测示例二", Status: "running", Version: 1,
			Description: "用于启动验证和主要流程演示的材料检测记录"}, Facility: "文物保护处理审批区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-512-02"},

		{BaseModel: model.BaseModel{Code: "MT-003", Name: "材料检测示例三", Status: "verified", Version: 1,
			Description: "用于启动验证和主要流程演示的材料检测记录"}, Facility: "文物保护处理审批区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-512-03"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedStageApproval(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.StageApproval{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.StageApproval{

		{BaseModel: model.BaseModel{Code: "SA-001", Name: "阶段审批示例一", Status: "draft", Version: 1,
			Description: "用于启动验证和主要流程演示的阶段审批记录"}, Facility: "文物保护处理审批区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-512-01"},

		{BaseModel: model.BaseModel{Code: "SA-002", Name: "阶段审批示例二", Status: "review", Version: 2,
			Description: "用于启动验证和主要流程演示的阶段审批记录"}, Facility: "文物保护处理审批区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-512-02"},

		{BaseModel: model.BaseModel{Code: "SA-003", Name: "阶段审批示例三", Status: "approved", Version: 1,
			Description: "用于启动验证和主要流程演示的阶段审批记录"}, Facility: "文物保护处理审批区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-512-03"},

		// 待补正：复核人已退回并填写意见，等待操作员提交补正说明。
		{BaseModel: model.BaseModel{Code: "SA-004", Name: "阶段审批示例四", Status: "correction", Version: 3,
			Description: "演示退回补正流程的阶段审批记录"}, Facility: "文物保护处理审批区域4", Owner: "现场操作员",
			Category: "补正", RiskLevel: "high", MetricValue: 42.0, MetricUnit: "score",
			EffectiveAt: now.Add(9 * time.Hour), Evidence: "检测影像页码待补充", RelatedCode: "REL-512-04"},
	}
	if err := db.WithContext(ctx).Create(&items).Error; err != nil {
		return err
	}
	opinions := []model.ApprovalOpinion{
		{StageApprovalID: items[1].ID, Version: 2, Batch: 1, Status: "review", Kind: model.OpinionKindSubmit,
			Opinion: "材料检测齐备，提交阶段复核", Actor: "operator", Role: model.RoleOperator, RequestID: "seed-sa002", CreatedAt: now},
		{StageApprovalID: items[3].ID, Version: 2, Batch: 1, Status: "review", Kind: model.OpinionKindSubmit,
			Opinion: "首次提交阶段复核", Actor: "operator", Role: model.RoleOperator, RequestID: "seed-sa004-submit", CreatedAt: now.Add(-2 * time.Hour)},
		{StageApprovalID: items[3].ID, Version: 3, Batch: 1, Status: "correction", Kind: model.OpinionKindDecision,
			Opinion: "检测报告缺少取样页码与影像编号，请补正后重新提交", Actor: "reviewer", Role: model.RoleReviewer, RequestID: "seed-sa004-correction", CreatedAt: now.Add(-1 * time.Hour)},
	}
	return db.WithContext(ctx).Create(&opinions).Error
}
