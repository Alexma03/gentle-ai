package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestReviewAcknowledgedEnvelopeMatchesPublishedSchema(t *testing.T) {
	schemaPath := filepath.Join("..", "..", "contracts", "review-integration", "v2", "schemas", "acknowledged.schema.json")
	payload, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	const schemaID = "https://gentle-ai.dev/contracts/review-integration/v2/schemas/acknowledged.schema.json"
	if err := compiler.AddResource(schemaID, document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(schemaID)
	if err != nil {
		t.Fatal(err)
	}

	valid := reviewAcknowledgedResult{
		Schema: reviewAcknowledgedSchema, Operation: "review/acknowledge-approved", Action: "acknowledged",
		LineageID: "review-lineage-1", TargetIdentity: "sha256:" + strings.Repeat("a", 64),
		ConsumedRevision: "sha256:" + strings.Repeat("b", 64), Authority: "burned",
	}
	var encoded bytes.Buffer
	if err := encodeReviewJSON(&encoded, valid); err != nil {
		t.Fatal(err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(instance); err != nil {
		t.Fatalf("emitted reviewAcknowledgedResult rejected by published schema: %v", err)
	}

	invalid := valid
	invalid.LineageID = strings.Repeat("a", 129)
	raw, err := json.Marshal(invalid)
	if err != nil {
		t.Fatal(err)
	}
	instance, err = jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(instance); err == nil {
		t.Fatal("published schema accepted a lineage rejected by canonical runtime validation")
	}
}
