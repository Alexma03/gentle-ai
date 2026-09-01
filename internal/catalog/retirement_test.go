package catalog

import "testing"

func TestComponentsExcludeRetiredInventory(t *testing.T) {
	for _, component := range MVPComponents() {
		switch string(component.ID) {
		case "gga", "theme", "claude-theme", "opencode-gentle-logo":
			t.Fatalf("MVPComponents() exposed retired component %#v", component)
		}
	}
}
