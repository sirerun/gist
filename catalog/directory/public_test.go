package directory

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPublicProjectionKeepsConnectionDetailsAndHidesIntakeEvidence(t *testing.T) {
	pin := "sha256:" + strings.Repeat("a", 64)
	record := Record{
		ID:                  "mcp:example",
		Kind:                "mcp",
		ServiceID:           "example",
		Title:               "Example Files",
		Description:         "Remote MCP endpoint listed by Make as a verified server. The operator is not verified by Gist. This reachability check is not an authenticated session or workflow qualification.",
		Tags:                []string{"files"},
		Source:              RecordSource{ID: "marketplace", EntryID: "private-source", URL: "https://www.make.com/private-source"},
		Remotes:             []Remote{{URL: "https://api.example/mcp", Transport: "streamable-http", Check: &EndpointCheck{CheckedAt: time.Now(), HTTPStatus: 401, Result: "authentication-required"}}},
		Auth:                Auth{Type: "oauth2", CredentialNames: []string{"EXAMPLE_TOKEN"}, SetupURL: "https://example.com/oauth"},
		Skills:              []Skill{{ID: "guide", Title: "Files guide", URL: "https://example.com/guide", Status: "candidate", SourceDigest: pin}},
		Verification:        []Qualification{{Client: "Zati", Workflow: "Read files", EvidenceURL: "https://gist.example/private-evidence", CheckedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), Result: "passed"}},
		Recommended:         true,
		Official:            false,
		OfficialEvidenceURL: "https://example.com/private-review",
		ServerListingURL:    "https://apps.make.com/private-listing",
		FeaturedSelection:   true,
		Featured:            true,
	}
	public := PublicRecordOf(record)
	raw, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, private := range []string{"marketplace", "private-source", "make.com", "check", "checked_at", "authentication-required", "verification", "private-evidence", "official_evidence_url", "recommended", "featured", "operator is not verified", "workflow qualification"} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(private)) {
			t.Fatalf("public descriptor exposed %q: %s", private, body)
		}
	}
	for _, useful := range []string{"Example Files", "files", "streamable-http", "oauth2", "EXAMPLE_TOKEN", "https://example.com/oauth", "Files guide", "https://example.com/guide", "candidate", pin} {
		if !strings.Contains(body, useful) {
			t.Fatalf("public descriptor omitted %q: %s", useful, body)
		}
	}
	if !strings.Contains(public.Description, "Example Files integration for files") {
		t.Fatalf("internal intake description not replaced: %q", public.Description)
	}
}

func TestPublicProjectionUsesUsefulFallbacks(t *testing.T) {
	if got := PublicRecordOf(Record{Kind: "openapi", Title: "Calendar"}).Description; !strings.Contains(got, "OpenAPI integration for Calendar") {
		t.Fatalf("OpenAPI fallback=%q", got)
	}
	if got := PublicRecordOf(Record{Kind: "mcp", Title: "Calendar"}).Description; !strings.Contains(got, "MCP server for Calendar") {
		t.Fatalf("MCP fallback=%q", got)
	}
}

func TestPublicProjectionStrictClientCompatibility(t *testing.T) {
	index, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range index.ids {
		original, _ := index.Get(id)
		projected := PublicRecordOf(original)
		raw, err := json.Marshal(projected)
		if err != nil {
			t.Fatal(err)
		}
		var record Record
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&record); err != nil {
			t.Fatalf("%s strict decode: %v", id, err)
		}
		if err := ValidateRecord(record); err != nil {
			t.Fatalf("%s client validation: %v", id, err)
		}
		if record.Source.ID != "gist" || record.Source.URL != "https://gist.sire.run/library" || record.Source.EntryID != id {
			t.Fatalf("%s source %+v", id, record.Source)
		}
		if record.Kind == "mcp" && (record.Official || record.OfficialEvidenceURL != "" || record.ServerListingURL != "https://gist.sire.run/library") {
			t.Fatalf("MCP inherited vendor claim: %+v", record)
		}
		if record.Kind == "openapi" && (record.Official != original.Official || record.OfficialEvidenceURL != original.OfficialEvidenceURL) {
			t.Fatal("OpenAPI vendor evidence changed")
		}
		if !reflect.DeepEqual(PublicResultOf(Result{Items: []Record{original}}).Items[0], projected) {
			t.Fatal("list/detail projection differs")
		}
	}
}
