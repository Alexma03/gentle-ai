package cli

import (
	"strings"
	"testing"

	componentuninstall "github.com/gentleman-programming/gentle-ai/v2/internal/components/uninstall"
)

// A batch that failed for one agent must not open with "complete". The user
// reads the header before the manual-cleanup detail printed further down.

func TestRenderUninstallReportHeaderStaysCompleteWhenNothingFailed(t *testing.T) {
	report := RenderUninstallReport(componentuninstall.Result{})
	header := strings.SplitN(report, "\n", 2)[0]
	if header != "Managed uninstall complete" {
		t.Fatalf("header = %q, want the unchanged complete header", header)
	}
}
