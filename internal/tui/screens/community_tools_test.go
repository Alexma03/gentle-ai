package screens

import (
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

func TestRenderCommunityToolsShowsCodeGraph(t *testing.T) {
	out := RenderCommunityTools([]model.CommunityToolID{model.CommunityToolCodeGraph}, 0, nil, false, nil)
	for _, want := range []string{"Community Tools/Plugins", "[x] CodeGraph", "View repo: https://github.com/colbymchenry/codegraph", "Continue", "Back"} {
		if !strings.Contains(out, want) {
			t.Fatalf("RenderCommunityTools missing %q; output:\n%s", want, out)
		}
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
