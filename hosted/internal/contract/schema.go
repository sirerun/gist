package contract

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// LoadJSONSchema parses and structurally validates a JSON Schema document.
// The schema is validated as JSON Schema 2020-12, rather than as the older
// OpenAPI 3.0 schema dialect.
func LoadJSONSchema(data []byte) (*openapi3.Schema, error) {
	schema := openapi3.NewSchema()
	if err := json.Unmarshal(data, schema); err != nil {
		return nil, fmt.Errorf("decode JSON Schema: %w", err)
	}
	if err := schema.Validate(context.Background(), openapi3.IsOpenAPI31OrLater()); err != nil {
		return nil, fmt.Errorf("validate JSON Schema: %w", err)
	}
	return schema, nil
}

// ValidateJSONSchema validates a JSON document against a JSON Schema
// 2020-12 document using kin-openapi's JSON Schema validator.
func ValidateJSONSchema(schemaData, documentData []byte) error {
	schema, err := LoadJSONSchema(schemaData)
	if err != nil {
		return err
	}
	var document any
	if err := json.Unmarshal(documentData, &document); err != nil {
		return fmt.Errorf("decode fixture JSON: %w", err)
	}
	if err := schema.VisitJSON(document, openapi3.EnableJSONSchema2020()); err != nil {
		return fmt.Errorf("validate fixture JSON: %w", err)
	}
	return nil
}
