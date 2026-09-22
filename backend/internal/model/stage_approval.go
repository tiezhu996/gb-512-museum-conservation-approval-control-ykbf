package model

import "time"

// StageApproval models 阶段审批 as an independently versioned aggregate. The fields
// cover ownership, operational context, evidence and measured risk so later
// changes naturally span persistence, service and UI layers.
type StageApproval struct {
	BaseModel
	Facility    string            `json:"facility" gorm:"size:120;index"`
	Owner       string            `json:"owner" gorm:"size:120;index"`
	Category    string            `json:"category" gorm:"size:80;index"`
	RiskLevel   string            `json:"riskLevel" gorm:"size:32;index"`
	MetricValue float64           `json:"metricValue"`
	MetricUnit  string            `json:"metricUnit" gorm:"size:24"`
	EffectiveAt time.Time         `json:"effectiveAt"`
	Evidence    string            `json:"evidence" gorm:"size:2000"`
	RelatedCode string            `json:"relatedCode" gorm:"size:64;index"`
	Opinions    []ApprovalOpinion `json:"opinions" gorm:"foreignKey:StageApprovalID;constraint:OnDelete:CASCADE"`
}

func (item *StageApproval) GetBase() *BaseModel { return &item.BaseModel }

func (item StageApproval) TableName() string { return "stage_approvals" }

var StageApprovalInitialStatus = "draft"

// ApprovalOpinion is append-only: one immutable opinion is stored for every
// aggregate version created by an approval state transition. Rows are never
// updated or deleted; Batch groups the review round (复核批次) the opinion
// belongs to so every request-correction/resubmission cycle stays traceable.
type ApprovalOpinion struct {
	ID              uint   `json:"id" gorm:"primaryKey"`
	StageApprovalID uint   `json:"stageApprovalId" gorm:"uniqueIndex:idx_approval_opinion_version;not null"`
	Version         uint   `json:"version" gorm:"uniqueIndex:idx_approval_opinion_version;not null"`
	Batch           uint   `json:"batch" gorm:"not null;default:1;index"`
	Status          string `json:"status" gorm:"size:40;not null"`
	Opinion         string `json:"opinion" gorm:"size:1000;not null"`
	// Kind distinguishes reviewer decisions (decision) from operator
	// correction submissions (correction) within the same append-only log.
	Kind      string    `json:"kind" gorm:"size:20;not null;default:decision"`
	Actor     string    `json:"actor" gorm:"size:80;not null;index"`
	Role      string    `json:"role" gorm:"size:32;not null;default:operator"`
	RequestID string    `json:"requestId" gorm:"size:64;not null;index"`
	CreatedAt time.Time `json:"createdAt" gorm:"index"`
}

// ApprovalOpinion kinds.
const (
	OpinionKindSubmit     = "submit"
	OpinionKindDecision   = "decision"
	OpinionKindCorrection = "correction"
)
