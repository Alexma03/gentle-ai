package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func TestRC1ConsecutiveRescopeProvenanceMatchesFixture(t *testing.T) {
	records, err := rc1ConsecutiveRescopeRecords()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 4 {
		t.Fatalf("provenance record count = %d, want 4", len(records))
	}
}

func TestPrintedConsecutiveRescopeRepairArgumentsPreservesLiteralBackticks(t *testing.T) {
	workspace := "/tmp/work`space"
	change := "repair`change"
	stderr := "published consecutive-rescope record sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa is unreadable under normal replay; " + consecutiveRescopeRepairPrefix +
		"gentle-ai sdd-attempt repair --cwd '" + workspace + "' --change '" + change + "'"

	got, err := printedConsecutiveRescopeRepairArguments(stderr)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"sdd-attempt", "repair", "--cwd", workspace, "--change", change}
	if !slices.Equal(got, want) {
		t.Fatalf("parsed repair command = %#v, want %#v", got, want)
	}
}

func TestRC1ConsecutiveRescopeProvenanceRefusesSameLengthOperationMutation(t *testing.T) {
	fixture := mutatedRC1ConsecutiveRescopeProvenance(t, func(manifest *rc1ConsecutiveRescopeManifest) {
		manifest.OperationShape[3] = "objective/rescope B to D"
	})
	if _, err := rc1ConsecutiveRescopeRecordsFrom(fixture); err == nil || !strings.Contains(err.Error(), "refuses a different ordered operation sequence") {
		t.Fatalf("same-length operation mutation = %v, want observable ordered-sequence refusal", err)
	}
}

func TestRC1ConsecutiveRescopeProvenanceRefusesGeneratorCommandMutation(t *testing.T) {
	fixture := mutatedRC1ConsecutiveRescopeProvenance(t, func(manifest *rc1ConsecutiveRescopeManifest) {
		manifest.GeneratorCommands[1] = "go build -o gentle-ai ./cmd/gentle-ai"
	})
	if _, err := rc1ConsecutiveRescopeRecordsFrom(fixture); err == nil || !strings.Contains(err.Error(), "refuses different generator commands") {
		t.Fatalf("generator command mutation = %v, want observable generator-command refusal", err)
	}
}

func TestRC1ConsecutiveRescopeProvenanceRefusesRecordMapMutation(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*rc1ConsecutiveRescopeManifest)
	}{
		{
			name: "missing expected record",
			mutate: func(manifest *rc1ConsecutiveRescopeManifest) {
				delete(manifest.Records, strings.TrimPrefix(rc1ConsecutiveRescopeHead, "sha256:")+".json")
			},
		},
		{
			name: "extra record",
			mutate: func(manifest *rc1ConsecutiveRescopeManifest) {
				manifest.Records["extra.json"] = "sha256:extra"
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := mutatedRC1ConsecutiveRescopeProvenance(t, test.mutate)
			if _, err := rc1ConsecutiveRescopeRecordsFrom(fixture); err == nil || !strings.Contains(err.Error(), "source-controlled expected records") {
				t.Fatalf("record map mutation = %v, want source-controlled record-set refusal", err)
			}
		})
	}
}

func TestRC1ConsecutiveRescopeProvenanceRefusesUnexpectedRecordReplacement(t *testing.T) {
	var name string
	payload := []byte("unexpected immutable record")
	fixture := mutatedRC1ConsecutiveRescopeProvenance(t, func(manifest *rc1ConsecutiveRescopeManifest) {
		delete(manifest.Records, strings.TrimPrefix(rc1ConsecutiveRescopeHead, "sha256:")+".json")
		sum := sha256.Sum256(payload)
		digest := "sha256:" + hex.EncodeToString(sum[:])
		name = strings.TrimPrefix(digest, "sha256:") + ".json"
		manifest.Records[name] = digest
	})
	fixture["testdata/consecutive-rescope-rc1/records/"+name] = &fstest.MapFile{Data: payload}
	if _, err := rc1ConsecutiveRescopeRecordsFrom(fixture); err == nil || !strings.Contains(err.Error(), "source-controlled expected records") {
		t.Fatalf("unexpected record replacement = %v, want source-controlled record-set refusal", err)
	}
}

func mutatedRC1ConsecutiveRescopeProvenance(t *testing.T, mutate func(*rc1ConsecutiveRescopeManifest)) fstest.MapFS {
	t.Helper()
	payload, err := consecutiveRescopeRC1.ReadFile("testdata/consecutive-rescope-rc1/provenance.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest rc1ConsecutiveRescopeManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatal(err)
	}
	mutate(&manifest)
	payload, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{
		"testdata/consecutive-rescope-rc1/provenance.json": &fstest.MapFile{Data: payload},
	}
	entries, err := consecutiveRescopeRC1.ReadDir("testdata/consecutive-rescope-rc1/records")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		path := "testdata/consecutive-rescope-rc1/records/" + entry.Name()
		record, err := consecutiveRescopeRC1.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		files[path] = &fstest.MapFile{Data: record}
	}
	return files
}
