package sddstatus

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/gentleman-programming/gentle-ai/v2/internal/consentenvelope"
	"github.com/gentleman-programming/gentle-ai/v2/internal/pathquote"
)

const SDDBudgetConsentSchema = "gentle-ai.sdd-integration.consent/v1"

// BudgetConsentInput preserves the legacy typed-envelope API. Routine runtime
// routing no longer constructs this question from accounting ceilings.
type BudgetConsentInput struct {
	Repo     string
	Change   string
	Revision string

	MaxAttempts        int
	CumulativeAttempts int
	MaxChangedLines    int
	CumulativeLines    int

	// HarnessFailures counts attempts whose harness result was invalidated.
	// Execution may have started and partial evidence may still exist.
	HarnessFailures int
}

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

const (
	sddBudgetConsentGranted  = "granted"
	sddBudgetConsentDeclined = "declined"
)

// BudgetConsentEnvelope builds the question. It refuses to produce an
// incomplete one: a question whose grant is not runnable verbatim is the exact
// defect it replaces, so the shared core's completeness rule is enforced here
// rather than trusted.
func BudgetConsentEnvelope(in BudgetConsentInput) (BudgetConsentResult, error) {
	if in.Repo == "" || in.Change == "" {
		return BudgetConsentResult{}, errors.New("budget consent requires the repository and change it is asking about") // refusal:by-design world-action: the envelope is built and validated by its producer; the exit is a code fix, not a command
	}
	if !runtimeRevisionPattern.MatchString(in.Revision) {
		// Without the exact revision the grant cannot be runnable verbatim,
		// and a grant the human has to complete by hand is the defect.
		return BudgetConsentResult{}, errors.New("budget consent requires the exact ledger revision the reset must name") // refusal:by-design world-action: the envelope is built and validated by its producer; the exit is a code fix, not a command
	}

	evidence := []string{
		fmt.Sprintf("attempts: %d of %d spent for this objective", in.CumulativeAttempts, in.MaxAttempts),
	}
	if in.MaxChangedLines > 0 {
		evidence = append(evidence, fmt.Sprintf("changed lines: %d of %d", in.CumulativeLines, in.MaxChangedLines))
	}

	headline := "This work unit has spent its attempt budget."
	reason := "the objective's bounded attempts are gone, so no further work unit can open until a maintainer decides."
	value := "Resetting opens a fresh bounded budget for this objective and keeps every recorded attempt in the immutable chain."

	if in.HarnessFailures > 0 {
		// The typed half. A prompt that only asks "retry?" reproduces #2588
		// with a click in the middle: its reporter answered that question four
		// times because nothing distinguished the tool failing from their code
		// failing. Saying so is what makes this answerable.
		headline = "This work unit spent its attempt budget with incomplete harness evidence."
		reason = fmt.Sprintf(
			"%d of the %d attempts were settled with an invalidated harness. Execution may have started and partial evidence may exist, but the harness result cannot prove the change.",
			in.HarnessFailures, in.CumulativeAttempts)
		evidence = append(evidence, fmt.Sprintf("attempts with incomplete or invalidated harness evidence: %d", in.HarnessFailures))
		if in.HarnessFailures > 1 {
			evidence = append(evidence,
				"more than one attempt failed the same way, which usually means the harness will not converge on a retry; a provided harness or a maintainer is the cheaper next step")
		}
	}

	core := consentenvelope.Core{
		Headline: headline, Reason: reason, Value: value, Evidence: evidence,
		Choices: []consentenvelope.Choice{
			{
				Answer: sddBudgetConsentGranted,
				Label:  "Open a fresh budget and keep going",
				Effect: "Resets this objective to a fresh bounded budget. Every attempt already recorded stays in the immutable chain, nothing is erased, and no verification, review or receipt is fabricated.",
				Invocation: fmt.Sprintf(
					"gentle-ai sdd-attempt reset --cwd %s --change %s --expected-revision %q --request-id %s --reason %q --actor %s",
					pathquote.Quote(in.Repo), in.Change, in.Revision,
					sddBudgetConsentResetRequestID(in), sddBudgetConsentReason(in), sddRuntimeAuditActor),
			},
			{
				Answer: sddBudgetConsentDeclined,
				Label:  "Stop here",
				Effect: "Leaves the objective exhausted and every attempt preserved. Nothing is reset and nothing is lost; the change simply does not continue until someone decides otherwise.",
				Invocation: fmt.Sprintf(
					"gentle-ai sdd-attempt status --cwd %s --change %s",
					pathquote.Quote(in.Repo), in.Change),
			},
		},
		OffPath: consentenvelope.OffPath{
			Note:    "Receipt-driven review is a separate switch and turning it off does NOT clear this: the attempt budget is SDD's own accounting, and review has no say in whether a work unit may open.",
			Command: fmt.Sprintf("gentle-ai review mode status --cwd %s", pathquote.Quote(in.Repo)),
		},
	}
	if err := core.ValidateCompleteness(sddBudgetConsentGranted, sddBudgetConsentDeclined); err != nil {
		return BudgetConsentResult{}, err
	}

	return BudgetConsentResult{
		Schema: SDDBudgetConsentSchema, Change: in.Change, Action: "consent_required", Blocking: true,
		Headline: core.Headline, Reason: core.Reason, Value: core.Value,
		Evidence: core.Evidence, Choices: core.Choices, OffPath: core.OffPath,
	}, nil
}

// sddRuntimeAuditActor is the actor the runtime records for a reset it
// materialized itself.
//
// The actor field is audit metadata, not authenticated human authority:
// nothing downstream treats it as proof of who decided. #2959 emitted a
// literal "<actor>" here, which forced the caller either to invent a value
// inside a provider-owned command the relay contract forbids rewriting, or to
// stop. A transparent fixed literal is honest about what the field is and
// keeps the invocation executable exactly as returned. The human decision
// itself is recorded by --reason, which still says a maintainer authorized it.
const sddRuntimeAuditActor = "gentle-ai-runtime"

// sddBudgetConsentResetRequestID derives the reset's request ID from the exact
// objective it resets: the change, the expected revision, and the operation.
//
// Deterministic rather than random, because the relay contract permits
// executing the returned command exactly once and a retry after an ambiguous
// outcome must be the SAME request, not a second budget. Bound to the expected
// revision, because a grant captured before authority moved must not be
// replayable onto the new state; --expected-revision already fails CAS there,
// and a distinct ID keeps the audit trail from claiming the stale decision was
// the current one. Hashed rather than composed from the change name, so the
// result satisfies runtimeRequestIDPattern for every change name that exists.
func sddBudgetConsentResetRequestID(in BudgetConsentInput) string {
	sum := sha256.Sum256([]byte("gentle-ai.sdd-budget-consent-reset/v1\x00" + in.Change + "\x00" + in.Revision))
	return "reset-" + hex.EncodeToString(sum[:16])
}

func sddBudgetConsentReason(in BudgetConsentInput) string {
	if in.HarnessFailures > 0 {
		return "maintainer authorized a fresh budget after incomplete harness evidence"
	}
	return "maintainer authorized a fresh budget for this objective"
}
