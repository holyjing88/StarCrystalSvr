package service

import "errors"

var (
	ErrTaskEmptyAccount   = errors.New("empty account id")
	ErrTaskNotFound       = errors.New("task not found or disabled")
	ErrTaskNotClaimable   = errors.New("task progress insufficient")
	ErrTaskAlreadyClaimed = errors.New("task already claimed")
	ErrTaskAdProofInvalid = errors.New("ad proof invalid or expired")
	ErrTaskNoEconomy      = errors.New("economy not configured")
)
