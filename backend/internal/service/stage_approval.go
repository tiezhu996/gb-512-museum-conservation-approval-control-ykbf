package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/museum-conservation-approval-control/backend/internal/constants"
	"github.com/blueship581/museum-conservation-approval-control/backend/internal/dto"
	"github.com/blueship581/museum-conservation-approval-control/backend/internal/model"
	"github.com/blueship581/museum-conservation-approval-control/backend/internal/repository"
)

type StageApprovalService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.StageApproval], error)
	Get(context.Context, uint) (model.StageApproval, error)
	Create(context.Context, dto.CreateStageApproval, string, string) (model.StageApproval, error)
	Update(context.Context, uint, dto.UpdateStageApproval, string, string) (model.StageApproval, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string, string) (model.StageApproval, error)
	SubmitCorrection(context.Context, uint, dto.CorrectionRequest, string, string, string) (model.StageApproval, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type stageApprovalService struct {
	repository repository.StageApprovalRepository
	security   SecurityService
}

func NewStageApprovalService(repo repository.StageApprovalRepository, security SecurityService) StageApprovalService {
	return &stageApprovalService{repository: repo, security: security}
}

func (s *stageApprovalService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.StageApproval], error) {
	return s.repository.List(ctx, query)
}

func (s *stageApprovalService) Get(ctx context.Context, id uint) (model.StageApproval, error) {
	return s.repository.Get(ctx, id)
}

func (s *stageApprovalService) Create(ctx context.Context, input dto.CreateStageApproval, actor, requestID string) (model.StageApproval, error) {
	if err := validateStageApprovalBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.StageApproval{}, err
	}
	item := model.StageApproval{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.StageApprovalInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.StageApproval{}, fmt.Errorf("create 阶段审批: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "StageApproval", item.ID, "", item.Status, "created 阶段审批")
	return item, nil
}

func (s *stageApprovalService) Update(ctx context.Context, id uint, input dto.UpdateStageApproval, actor, requestID string) (model.StageApproval, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.StageApproval{}, err
	}
	// Business fields stay immutable once review starts. The 待补正 state is not
	// an editable state either: corrections are submitted as explanations and
	// recorded in the append-only opinion log.
	if current.Status != string(constants.ApprovalStateDraft) {
		return model.StageApproval{}, ErrApprovalLocked
	}
	if err := validateStageApprovalBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.StageApproval{}, err
	}
	current.Name = strings.TrimSpace(input.Name)
	current.Description = strings.TrimSpace(input.Description)
	current.Facility = strings.TrimSpace(input.Facility)
	current.Owner = strings.TrimSpace(input.Owner)
	current.Category = strings.TrimSpace(input.Category)
	current.RiskLevel = input.RiskLevel
	current.MetricValue = input.MetricValue
	current.MetricUnit = strings.TrimSpace(input.MetricUnit)
	current.EffectiveAt = input.EffectiveAt.UTC()
	current.Evidence = strings.TrimSpace(input.Evidence)
	current.RelatedCode = strings.ToUpper(strings.TrimSpace(input.RelatedCode))
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.StageApproval{}, fmt.Errorf("update 阶段审批: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "StageApproval", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

// reviewerDecisions are the transitions only a reviewer or administrator may perform.
var reviewerDecisions = map[string]bool{
	string(constants.ApprovalStateApproved):   true,
	string(constants.ApprovalStateRejected):   true,
	string(constants.ApprovalStateCorrection): true,
}

func isReviewerRole(role string) bool {
	return role == model.RoleReviewer || role == model.RoleAdmin
}

// currentBatch derives the review batch (复核批次) number from the append-only
// opinion log. Opinions are stored version-ascending; every resubmission after
// a 退回补正 opens a new batch.
func currentBatch(opinions []model.ApprovalOpinion) uint {
	var batch uint = 1
	for _, opinion := range opinions {
		if opinion.Kind == model.OpinionKindCorrection {
			batch = opinion.Batch
		}
	}
	return batch
}

func (s *stageApprovalService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, role, requestID string) (model.StageApproval, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.StageApproval{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.StageApprovalTransitions, current.Status, target) {
		return model.StageApproval{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	if reviewerDecisions[target] && !isReviewerRole(role) {
		return model.StageApproval{}, ErrReviewerRequired
	}
	before := current.Status
	now := time.Now().UTC()
	batch := currentBatch(current.Opinions)
	kind := model.OpinionKindDecision
	// A direct draft -> review submission starts the first review batch; later
	// review entries come from correction resubmissions (handled separately).
	if target == string(constants.ApprovalStateReview) {
		kind = model.OpinionKindSubmit
	}
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = now
	opinion := &model.ApprovalOpinion{
		Version: input.ExpectedVersion + 1, Batch: batch, Status: target,
		Opinion: strings.TrimSpace(input.Reason), Kind: kind,
		Actor: actor, Role: role, RequestID: requestID, CreatedAt: now,
	}
	if err := s.repository.TransitionWithOpinion(ctx, id, input.ExpectedVersion, &current, opinion); err != nil {
		return model.StageApproval{}, fmt.Errorf("transition 阶段审批: %w", err)
	}
	action := "transition"
	if target == string(constants.ApprovalStateCorrection) {
		action = "request_correction"
	}
	if err := s.security.Audit(ctx, actor, requestID, action, "StageApproval", id, before, target, input.Reason); err != nil {
		return model.StageApproval{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

// SubmitCorrection handles an operator explanation for an approval awaiting
// correction. It appends an immutable correction opinion carrying the actor,
// role and request ID, moves the approval back to review and opens the next
// review batch; the reviewer still decides approve or reject there.
func (s *stageApprovalService) SubmitCorrection(ctx context.Context, id uint, input dto.CorrectionRequest, actor, role, requestID string) (model.StageApproval, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.StageApproval{}, err
	}
	if current.Status != string(constants.ApprovalStateCorrection) {
		return model.StageApproval{}, ErrCorrectionRequired
	}
	if role != model.RoleOperator {
		return model.StageApproval{}, ErrOperatorRequired
	}
	explanation := strings.TrimSpace(input.Correction)
	if explanation == "" {
		return model.StageApproval{}, ErrInvalidInput
	}
	before := current.Status
	now := time.Now().UTC()
	nextBatch := currentBatch(current.Opinions) + 1
	target := string(constants.ApprovalStateReview)
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = now
	opinion := &model.ApprovalOpinion{
		Version: input.ExpectedVersion + 1, Batch: nextBatch, Status: target,
		Opinion: explanation, Kind: model.OpinionKindCorrection,
		Actor: actor, Role: role, RequestID: requestID, CreatedAt: now,
	}
	if err := s.repository.TransitionWithOpinion(ctx, id, input.ExpectedVersion, &current, opinion); err != nil {
		return model.StageApproval{}, fmt.Errorf("submit correction 阶段审批: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "submit_correction", "StageApproval", id, before, target, explanation); err != nil {
		return model.StageApproval{}, fmt.Errorf("persist correction audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *stageApprovalService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "StageApproval", id, current.Status, "deleted", "soft deleted 阶段审批")
}

func (s *stageApprovalService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateStageApprovalBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
