package middleware

import (
	"github.com/Tencent/WeKnora/internal/application/service"
)

// TokenQuotaChecker is a convenience wrapper for token quota check and record operations.
// It is designed to be used inline within chat/agent handlers rather than as a wrapping middleware,
// because token estimation depends on the specific request body which only the handler knows.
//
// Usage in handler:
//
//	// Before LLM call
//	estimated := utils.EstimateTokensWithBuffer(prompt)
//	if err := quotaChecker.CheckQuota(ctx, tenantID, estimated); err != nil {
//	    // return quota exceeded error to client
//	}
//
//	// After LLM call
//	quotaChecker.RecordUsage(ctx, tenantID, response.Usage.TotalTokens)
type TokenQuotaChecker struct {
	svc *service.TokenQuotaService
}

// NewTokenQuotaChecker creates a new token quota checker
func NewTokenQuotaChecker(svc *service.TokenQuotaService) *TokenQuotaChecker {
	return &TokenQuotaChecker{svc: svc}
}

// Service returns the underlying TokenQuotaService for direct use
func (c *TokenQuotaChecker) Service() *service.TokenQuotaService {
	return c.svc
}
