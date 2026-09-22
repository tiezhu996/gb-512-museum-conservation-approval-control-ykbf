package service

import "errors"

var (
	ErrInvalidTransition  = errors.New("requested status transition is not allowed")
	ErrInvalidInput       = errors.New("business input validation failed")
	ErrUnauthorized       = errors.New("invalid username or password")
	ErrInactiveUser       = errors.New("user account is inactive")
	ErrApprovalLocked     = errors.New("approval fields are immutable after review starts")
	ErrReviewerRequired   = errors.New("a reviewer or administrator must approve, reject or return for correction")
	ErrOperatorRequired   = errors.New("an operator or administrator must submit the correction")
	ErrCorrectionOnly     = errors.New("a correction note can only be attached while the approval is pending correction")
	ErrOpinionRequired    = errors.New("a non-empty opinion is required for this action")
)
