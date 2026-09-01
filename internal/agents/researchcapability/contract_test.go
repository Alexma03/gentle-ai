package researchcapability

import (
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

func TestDocumentationLikeInputsNeverBecomeGrants(t *testing.T) {
	t.Parallel()

	for _, input := range []Grant{
		"requirements.txt",
		"CMakeLists.txt",
		"guide.md --execute",
		"component.mdx --run",
		"README.sh",
	} {
		input := input
		t.Run(string(input), func(t *testing.T) {
			t.Parallel()
			got := Admit(Request{
				Schema: SchemaV1, AgentID: model.AgentClaudeCode,
				Class: ClassDocumentation, ObservedGrants: []Grant{input},
			})
			if got.Allowed || len(got.VerifiedGrants) != 0 || len(got.Claims) != 0 {
				t.Fatalf("Admit(%q) = %#v, want closed denial without grants or claims", input, got)
			}
		})
	}
}
