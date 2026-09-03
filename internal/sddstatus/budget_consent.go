package sddstatus

import "github.com/gentleman-programming/gentle-ai/v2/internal/consentenvelope"

// BudgetConsentResult is retained for schema and API compatibility. Routine
// accounting exhaustion no longer routes through this blocking question.
type BudgetConsentResult struct {
	Schema   string `json:"schema"`
	Change   string `json:"change"`
	Action   string `json:"action"`
	Blocking bool   `json:"blocking"`

	Headline string                   `json:"headline"`
	Reason   string                   `json:"reason"`
	Value    string                   `json:"value"`
	Evidence []string                 `json:"evidence"`
	Choices  []consentenvelope.Choice `json:"choices"`
	OffPath  consentenvelope.OffPath  `json:"off_path"`
}
