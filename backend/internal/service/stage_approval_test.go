package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/blueship581/museum-conservation-approval-control/backend/internal/config"
	"github.com/blueship581/museum-conservation-approval-control/backend/internal/dto"
	"github.com/blueship581/museum-conservation-approval-control/backend/internal/model"
	"github.com/blueship581/museum-conservation-approval-control/backend/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newStageApprovalTestService(t *testing.T) (StageApprovalService, repository.StageApprovalRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&model.StageApproval{}, &model.ApprovalOpinion{}, &model.AuditLog{}, &model.User{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	approvalRepository := repository.NewStageApprovalRepository(db)
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	return NewStageApprovalService(approvalRepository, security), approvalRepository
}

func seedDraftApproval(t *testing.T, repo repository.StageApprovalRepository) model.StageApproval {
	t.Helper()
	item := model.StageApproval{
		BaseModel: model.BaseModel{Code: "SA-TEST", Name: "Test approval", Status: "draft", Version: 1},
		Facility:  "Conservation Lab", Owner: "operator", Category: "treatment", RiskLevel: "medium",
		EffectiveAt: time.Now().UTC(), Evidence: "material test attached", RelatedCode: "TP-TEST",
	}
	if err := repo.Create(context.Background(), &item); err != nil {
		t.Fatalf("create approval: %v", err)
	}
	return item
}

func TestStageApprovalAppendsImmutableOpinionVersions(t *testing.T) {
	svc, approvalRepository := newStageApprovalTestService(t)
	item := seedDraftApproval(t, approvalRepository)

	review, err := svc.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: 1, Reason: "材料检测通过，提交阶段复核",
	}, "operator", model.RoleOperator, "request-review")
	if err != nil {
		t.Fatalf("submit review: %v", err)
	}
	if review.Status != "review" || len(review.Opinions) != 1 {
		t.Fatalf("unexpected review state: %+v", review)
	}
	if review.Opinions[0].Version != 2 || review.Opinions[0].Batch != 1 ||
		review.Opinions[0].Actor != "operator" || review.Opinions[0].RequestID != "request-review" {
		t.Fatalf("review opinion did not preserve version context: %+v", review.Opinions[0])
	}

	_, err = svc.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "approved", ExpectedVersion: review.Version, Reason: "operator attempted approval",
	}, "operator", model.RoleOperator, "request-denied")
	if !errors.Is(err, ErrReviewerRequired) {
		t.Fatalf("expected reviewer role error, got %v", err)
	}

	approved, err := svc.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "approved", ExpectedVersion: review.Version, Reason: "检测证据完整，同意进入下一阶段",
	}, "reviewer", model.RoleReviewer, "request-approved")
	if err != nil {
		t.Fatalf("approve stage: %v", err)
	}
	if approved.Status != "approved" || len(approved.Opinions) != 2 {
		t.Fatalf("unexpected approved state: %+v", approved)
	}
	if approved.Opinions[0].Opinion != "材料检测通过，提交阶段复核" || approved.Opinions[1].Version != 3 ||
		approved.Opinions[1].Actor != "reviewer" || approved.Opinions[1].RequestID != "request-approved" {
		t.Fatalf("opinion history was overwritten or incomplete: %+v", approved.Opinions)
	}

	_, err = svc.Update(context.Background(), item.ID, dto.UpdateStageApproval{ExpectedVersion: approved.Version}, "admin", "request-update")
	if !errors.Is(err, ErrApprovalLocked) {
		t.Fatalf("expected immutable approval error, got %v", err)
	}
}

func TestReturnForCorrectionOpensNewReviewBatch(t *testing.T) {
	svc, repo := newStageApprovalTestService(t)
	item := seedDraftApproval(t, repo)
	ctx := context.Background()

	review, err := svc.Transition(ctx, item.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: 1, Reason: "提交复核",
	}, "operator", model.RoleOperator, "req-1")
	if err != nil {
		t.Fatalf("submit review: %v", err)
	}

	// reviewer demands 补正 with an opinion; approval becomes pending_correction.
	returned, err := svc.ReturnForCorrection(ctx, item.ID, dto.ReturnForCorrectionRequest{
		ExpectedVersion: review.Version, Opinion: "证据缺少比例尺，请补正影像与温度记录",
	}, "reviewer", model.RoleReviewer, "req-return")
	if err != nil {
		t.Fatalf("return for correction: %v", err)
	}
	if returned.Status != "pending_correction" || returned.Version != review.Version+1 {
		t.Fatalf("unexpected returned state: %+v", returned)
	}
	last := returned.Opinions[len(returned.Opinions)-1]
	if last.Batch != 1 || last.Status != "pending_correction" || last.Actor != "reviewer" || last.Opinion == "" {
		t.Fatalf("return opinion missing required context: %+v", last)
	}

	// operator submits the correction note; a new review batch opens.
	corrected, err := svc.SubmitCorrection(ctx, item.ID, dto.CorrectionRequest{
		ExpectedVersion: returned.Version, Opinion: "已补齐比例尺影像与完整温度记录",
	}, "operator", model.RoleOperator, "req-correct")
	if err != nil {
		t.Fatalf("submit correction: %v", err)
	}
	if corrected.Status != "review" || corrected.Version != returned.Version+1 {
		t.Fatalf("unexpected corrected state: %+v", corrected)
	}
	correctionOpinion := corrected.Opinions[len(corrected.Opinions)-1]
	if correctionOpinion.Batch != 2 || correctionOpinion.Actor != "operator" {
		t.Fatalf("correction must open batch 2: %+v", correctionOpinion)
	}

	// reviewer still decides pass or reject on the new batch.
	approved, err := svc.Transition(ctx, item.ID, dto.TransitionRequest{
		Status: "approved", ExpectedVersion: corrected.Version, Reason: "补正充分，复核通过",
	}, "reviewer", model.RoleReviewer, "req-approve")
	if err != nil {
		t.Fatalf("approve after correction: %v", err)
	}
	if approved.Status != "approved" {
		t.Fatalf("expected approved, got %s", approved.Status)
	}

	// Every opinion, actor, request id, original text and batch is preserved.
	if len(approved.Opinions) != 4 {
		t.Fatalf("expected four immutable opinions, got %d", len(approved.Opinions))
	}
	want := []struct {
		version, batch uint
		status, actor  string
	}{
		{2, 1, "review", "operator"},
		{3, 1, "pending_correction", "reviewer"},
		{4, 2, "review", "operator"},
		{5, 2, "approved", "reviewer"},
	}
	for index, expected := range want {
		got := approved.Opinions[index]
		if got.Version != expected.version || got.Batch != expected.batch ||
			got.Status != expected.status || got.Actor != expected.actor {
			t.Fatalf("opinion %d drifted: got %+v want %+v", index, got, expected)
		}
	}
}

func TestCorrectionRejectsUnauthorizedRole(t *testing.T) {
	svc, repo := newStageApprovalTestService(t)
	item := seedDraftApproval(t, repo)
	ctx := context.Background()
	review, err := svc.Transition(ctx, item.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: 1, Reason: "提交复核",
	}, "operator", model.RoleOperator, "req-1")
	if err != nil {
		t.Fatalf("submit review: %v", err)
	}

	// operator cannot demand 补正 while the approval is under review.
	if _, err := svc.ReturnForCorrection(ctx, item.ID, dto.ReturnForCorrectionRequest{
		ExpectedVersion: review.Version, Opinion: "操作员尝试退回",
	}, "operator", model.RoleOperator, "req-denied-return"); !errors.Is(err, ErrReviewerRequired) {
		t.Fatalf("operator return must fail with reviewer error, got %v", err)
	}

	returned, err := svc.ReturnForCorrection(ctx, item.ID, dto.ReturnForCorrectionRequest{
		ExpectedVersion: review.Version, Opinion: "需要补正证据",
	}, "admin", model.RoleAdmin, "req-return")
	if err != nil {
		t.Fatalf("admin may return for correction: %v", err)
	}
	// reviewer cannot self-clear 补正.
	if _, err := svc.SubmitCorrection(ctx, item.ID, dto.CorrectionRequest{
		ExpectedVersion: returned.Version, Opinion: "复核人尝试自行补正",
	}, "reviewer", model.RoleReviewer, "req-denied-correct"); !errors.Is(err, ErrOperatorRequired) {
		t.Fatalf("reviewer correction must fail with operator error, got %v", err)
	}
	// admin may file the correction on behalf of operations.
	if _, err := svc.SubmitCorrection(ctx, item.ID, dto.CorrectionRequest{
		ExpectedVersion: returned.Version, Opinion: "管理员代办补正",
	}, "admin", model.RoleAdmin, "req-admin-correct"); err != nil {
		t.Fatalf("admin correction should be allowed, got %v", err)
	}
}

func TestCorrectionOnlyAllowedWhilePending(t *testing.T) {
	svc, repo := newStageApprovalTestService(t)
	item := seedDraftApproval(t, repo)
	ctx := context.Background()

	// returning from a non-review state fails as an invalid transition.
	if _, err := svc.ReturnForCorrection(ctx, item.ID, dto.ReturnForCorrectionRequest{
		ExpectedVersion: 1, Opinion: "draft 记录无法退回补正",
	}, "reviewer", model.RoleReviewer, "req-return-draft"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("return on draft must fail, got %v", err)
	}

	// correction note against a draft record must fail even though draft -> review
	// is a legal generic transition: only 待补正 records accept a correction.
	if _, err := svc.SubmitCorrection(ctx, item.ID, dto.CorrectionRequest{
		ExpectedVersion: 1, Opinion: "尚未退回就尝试补正",
	}, "operator", model.RoleOperator, "req-too-early"); !errors.Is(err, ErrCorrectionOnly) {
		t.Fatalf("correction on draft must fail, got %v", err)
	}

	review, err := svc.Transition(ctx, item.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: 1, Reason: "提交复核",
	}, "operator", model.RoleOperator, "req-review")
	if err != nil {
		t.Fatalf("submit review: %v", err)
	}
	// correction note while still under review is also rejected.
	if _, err := svc.SubmitCorrection(ctx, item.ID, dto.CorrectionRequest{
		ExpectedVersion: review.Version, Opinion: "未退回先补正",
	}, "operator", model.RoleOperator, "req-still-review"); !errors.Is(err, ErrCorrectionOnly) {
		t.Fatalf("correction on review must fail, got %v", err)
	}
}

func TestStaleVersionAndPermissionFailuresDoNotMutate(t *testing.T) {
	svc, repo := newStageApprovalTestService(t)
	item := seedDraftApproval(t, repo)
	ctx := context.Background()
	review, err := svc.Transition(ctx, item.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: 1, Reason: "提交复核",
	}, "operator", model.RoleOperator, "req-review")
	if err != nil {
		t.Fatalf("submit review: %v", err)
	}
	returned, err := svc.ReturnForCorrection(ctx, item.ID, dto.ReturnForCorrectionRequest{
		ExpectedVersion: review.Version, Opinion: "需要补正",
	}, "reviewer", model.RoleReviewer, "req-return")
	if err != nil {
		t.Fatalf("return: %v", err)
	}
	opinionsBefore := len(returned.Opinions)

	// stale version must fail and must not append any opinion or change status.
	if _, err := svc.SubmitCorrection(ctx, item.ID, dto.CorrectionRequest{
		ExpectedVersion: review.Version, Opinion: "用过期版本补正",
	}, "operator", model.RoleOperator, "req-stale"); !errors.Is(err, repository.ErrVersionConflict) {
		t.Fatalf("stale correction must fail with version conflict, got %v", err)
	}
	// permission failure must fail identically without touching data.
	if _, err := svc.SubmitCorrection(ctx, item.ID, dto.CorrectionRequest{
		ExpectedVersion: returned.Version, Opinion: "无权限补正",
	}, "reviewer", model.RoleReviewer, "req-forbidden"); !errors.Is(err, ErrOperatorRequired) {
		t.Fatalf("unauthorized correction must fail, got %v", err)
	}

	after, err := repo.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("reload approval: %v", err)
	}
	if after.Status != "pending_correction" || after.Version != returned.Version {
		t.Fatalf("record mutated after failed submissions: %+v", after)
	}
	if len(after.Opinions) != opinionsBefore {
		t.Fatalf("failed submissions appended opinions: before=%d after=%d", opinionsBefore, len(after.Opinions))
	}
}

func TestCorrectionOpinionIsMandatory(t *testing.T) {
	svc, repo := newStageApprovalTestService(t)
	item := seedDraftApproval(t, repo)
	ctx := context.Background()
	review, err := svc.Transition(ctx, item.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: 1, Reason: "提交复核",
	}, "operator", model.RoleOperator, "req-review")
	if err != nil {
		t.Fatalf("submit review: %v", err)
	}
	returned, err := svc.ReturnForCorrection(ctx, item.ID, dto.ReturnForCorrectionRequest{
		ExpectedVersion: review.Version, Opinion: "需要补正",
	}, "reviewer", model.RoleReviewer, "req-return")
	if err != nil {
		t.Fatalf("return: %v", err)
	}
	if _, err := svc.SubmitCorrection(ctx, item.ID, dto.CorrectionRequest{
		ExpectedVersion: returned.Version, Opinion: "   ",
	}, "operator", model.RoleOperator, "req-blank"); !errors.Is(err, ErrOpinionRequired) {
		t.Fatalf("blank correction note must fail, got %v", err)
	}
}
