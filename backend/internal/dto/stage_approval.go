package dto

import "time"

// CreateStageApproval is the public write contract for 阶段审批. Status is deliberately
// omitted so callers cannot bypass the service state machine.
type CreateStageApproval struct {
	Code        string    `json:"code" binding:"required,min=2,max=64"`
	Name        string    `json:"name" binding:"required,min=2,max=160"`
	Description string    `json:"description" binding:"max=1000"`
	Facility    string    `json:"facility" binding:"required,max=120"`
	Owner       string    `json:"owner" binding:"required,max=120"`
	Category    string    `json:"category" binding:"required,max=80"`
	RiskLevel   string    `json:"riskLevel" binding:"required,oneof=low medium high critical"`
	MetricValue float64   `json:"metricValue"`
	MetricUnit  string    `json:"metricUnit" binding:"max=24"`
	EffectiveAt time.Time `json:"effectiveAt" binding:"required"`
	Evidence    string    `json:"evidence" binding:"max=2000"`
	RelatedCode string    `json:"relatedCode" binding:"max=64"`
}

type UpdateStageApproval struct {
	ExpectedVersion uint      `json:"expectedVersion" binding:"required"`
	Name            string    `json:"name" binding:"required,min=2,max=160"`
	Description     string    `json:"description" binding:"max=1000"`
	Facility        string    `json:"facility" binding:"required,max=120"`
	Owner           string    `json:"owner" binding:"required,max=120"`
	Category        string    `json:"category" binding:"required,max=80"`
	RiskLevel       string    `json:"riskLevel" binding:"required,oneof=low medium high critical"`
	MetricValue     float64   `json:"metricValue"`
	MetricUnit      string    `json:"metricUnit" binding:"max=24"`
	EffectiveAt     time.Time `json:"effectiveAt" binding:"required"`
	Evidence        string    `json:"evidence" binding:"max=2000"`
	RelatedCode     string    `json:"relatedCode" binding:"max=64"`
}

// ReturnForCorrectionRequest is used by a reviewer or administrator to send an
// approval currently under review back to the operator for 补正. The opinion is
// mandatory: it explains what must be corrected and is kept forever.
type ReturnForCorrectionRequest struct {
	ExpectedVersion uint   `json:"expectedVersion" binding:"required"`
	Opinion         string `json:"opinion" binding:"required,min=3,max=500"`
}

// CorrectionRequest is used by the operator after 待补正. The correction note
// opens a new review batch but never rewrites earlier opinions or fields.
type CorrectionRequest struct {
	ExpectedVersion uint   `json:"expectedVersion" binding:"required"`
	Opinion         string `json:"opinion" binding:"required,min=3,max=500"`
}
