package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/blueship581/museum-conservation-approval-control/backend/internal/config"
	"github.com/blueship581/museum-conservation-approval-control/backend/internal/constants"
	"github.com/blueship581/museum-conservation-approval-control/backend/internal/dto"
	"github.com/blueship581/museum-conservation-approval-control/backend/internal/model"
	"github.com/blueship581/museum-conservation-approval-control/backend/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newApprovalTestService(t *testing.T) (StageApprovalService, repository.StageApprovalRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&model.StageApproval{}, &model.ApprovalOpinion{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	approvalRepository := repository.NewStageApprovalRepository(db)
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	return NewStageApprovalService(approvalRepository, security), approvalRepository
}

func createDraftApproval(t *testing.T, repo repository.StageApprovalRepository) model.StageApproval {
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
	svc, repo := newApprovalTestService(t)
	item := createDraftApproval(t, repo)

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
		review.Opinions[0].Actor != "operator" || review.Opinions[0].RequestID != "request-review" ||
		review.Opinions[0].Kind != model.OpinionKindSubmit {
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
		approved.Opinions[1].Batch != 1 || approved.Opinions[1].Kind != model.OpinionKindDecision ||
		approved.Opinions[1].Actor != "reviewer" || approved.Opinions[1].RequestID != "request-approved" {
		t.Fatalf("opinion history was overwritten or incomplete: %+v", approved.Opinions)
	}

	_, err = svc.Update(context.Background(), item.ID, dto.UpdateStageApproval{ExpectedVersion: approved.Version}, "admin", "request-update")
	if !errors.Is(err, ErrApprovalLocked) {
		t.Fatalf("expected immutable approval error, got %v", err)
	}
}

func TestStageApprovalCorrectionRoundPreservesHistoryAndBatches(t *testing.T) {
	svc, repo := newApprovalTestService(t)
	item := createDraftApproval(t, repo)
	ctx := context.Background()

	review, err := svc.Transition(ctx, item.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: 1, Reason: "提交首次复核",
	}, "operator", model.RoleOperator, "req-submit-1")
	if err != nil {
		t.Fatalf("submit review: %v", err)
	}

	// Reviewer requests correction with an opinion; approval awaits correction.
	correction, err := svc.Transition(ctx, item.ID, dto.TransitionRequest{
		Status: string(constants.ApprovalStateCorrection), ExpectedVersion: review.Version, Reason: "证据编号缺失，请补正材料检测页码",
	}, "reviewer", model.RoleReviewer, "req-correction")
	if err != nil {
		t.Fatalf("request correction: %v", err)
	}
	if correction.Status != "correction" || len(correction.Opinions) != 2 {
		t.Fatalf("unexpected correction state: %+v", correction)
	}
	if correction.Opinions[1].Batch != 1 || correction.Opinions[1].Kind != model.OpinionKindDecision {
		t.Fatalf("request-correction opinion misclassified: %+v", correction.Opinions[1])
	}

	// Operator cannot approve while awaiting correction.
	if _, err := svc.Transition(ctx, item.ID, dto.TransitionRequest{
		Status: "approved", ExpectedVersion: correction.Version, Reason: "operator attempted approval",
	}, "operator", model.RoleOperator, "req-deny"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition from correction, got %v", err)
	}

	// Reviewer cannot submit the correction explanation themselves.
	if _, err := svc.SubmitCorrection(ctx, item.ID, dto.CorrectionRequest{
		ExpectedVersion: correction.Version, Correction: "复核员代填补正",
	}, "reviewer", model.RoleReviewer, "req-role-deny"); !errors.Is(err, ErrOperatorRequired) {
		t.Fatalf("expected operator required error, got %v", err)
	}

	// A stale version must fail without changing any data.
	if _, err := svc.SubmitCorrection(ctx, item.ID, dto.CorrectionRequest{
		ExpectedVersion: review.Version, Correction: "基于过期版本的补正说明",
	}, "operator", model.RoleOperator, "req-stale"); !errors.Is(err, repository.ErrVersionConflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}
	afterStale, err := svc.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("reload approval: %v", err)
	}
	if afterStale.Status != "correction" || afterStale.Version != correction.Version || len(afterStale.Opinions) != 2 {
		t.Fatalf("stale submission must not change data: %+v", afterStale)
	}

	// Operator submits the correction explanation, opening a new review batch.
	resubmitted, err := svc.SubmitCorrection(ctx, item.ID, dto.CorrectionRequest{
		ExpectedVersion: correction.Version, Correction: "已补充检测页码 MT-002 p.7 并重新扫描证据",
	}, "operator", model.RoleOperator, "req-resubmit")
	if err != nil {
		t.Fatalf("submit correction: %v", err)
	}
	if resubmitted.Status != "review" || len(resubmitted.Opinions) != 3 {
		t.Fatalf("unexpected resubmitted state: %+v", resubmitted)
	}
	last := resubmitted.Opinions[2]
	if last.Batch != 2 || last.Kind != model.OpinionKindCorrection || last.Actor != "operator" ||
		last.Role != model.RoleOperator || last.RequestID != "req-resubmit" ||
		last.Opinion != "已补充检测页码 MT-002 p.7 并重新扫描证据" {
		t.Fatalf("correction opinion missing actor/batch/content: %+v", last)
	}

	// Reviewer approves within the new batch; all earlier opinions survive.
	approved, err := svc.Transition(ctx, item.ID, dto.TransitionRequest{
		Status: "approved", ExpectedVersion: resubmitted.Version, Reason: "补正充分，复核通过",
	}, "reviewer", model.RoleReviewer, "req-approve-2")
	if err != nil {
		t.Fatalf("approve after correction: %v", err)
	}
	if approved.Status != "approved" || len(approved.Opinions) != 4 {
		t.Fatalf("unexpected final state: %+v", approved)
	}
	final := approved.Opinions[3]
	if final.Batch != 2 || final.Version != 5 || final.Opinion != "补正充分，复核通过" {
		t.Fatalf("second-batch decision misrecorded: %+v", final)
	}
	original := approved.Opinions[0]
	if original.Opinion != "提交首次复核" || original.Actor != "operator" {
		t.Fatalf("original content must remain immutable: %+v", original)
	}

	// No correction submission is possible after approval.
	if _, err := svc.SubmitCorrection(ctx, item.ID, dto.CorrectionRequest{
		ExpectedVersion: approved.Version, Correction: "终态后补正",
	}, "operator", model.RoleOperator, "req-final"); !errors.Is(err, ErrCorrectionRequired) {
		t.Fatalf("expected correction state error, got %v", err)
	}
}

func TestStageApprovalRejectAndSecondCorrectionCycle(t *testing.T) {
	svc, repo := newApprovalTestService(t)
	item := createDraftApproval(t, repo)
	ctx := context.Background()

	review, _ := svc.Transition(ctx, item.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: 1, Reason: "提交复核",
	}, "operator", model.RoleOperator, "r1")
	correction, _ := svc.Transition(ctx, item.ID, dto.TransitionRequest{
		Status: "correction", ExpectedVersion: review.Version, Reason: "请补正",
	}, "reviewer", model.RoleReviewer, "r2")
	resubmitted, _ := svc.SubmitCorrection(ctx, item.ID, dto.CorrectionRequest{
		ExpectedVersion: correction.Version, Correction: "第一次补正说明",
	}, "operator", model.RoleOperator, "r3")

	// Reviewer can reject within the new review batch.
	rejected, err := svc.Transition(ctx, item.ID, dto.TransitionRequest{
		Status: "rejected", ExpectedVersion: resubmitted.Version, Reason: "补正仍不充分，驳回",
	}, "reviewer", model.RoleReviewer, "r4")
	if err != nil {
		t.Fatalf("reject second batch: %v", err)
	}
	if rejected.Status != "rejected" || rejected.Opinions[3].Batch != 2 {
		t.Fatalf("unexpected rejection: %+v", rejected)
	}
}
