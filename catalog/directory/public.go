package directory

import (
	"fmt"
	"strings"
)

// PublicRecordOf copies user-facing connection metadata while dropping the
// private intake, reachability and qualification details held by Record.
func PublicRecordOf(record Record) PublicRecord {
	remotes := make([]PublicRemote, 0, len(record.Remotes))
	for _, remote := range record.Remotes {
		remotes = append(remotes, PublicRemote{URL: remote.URL, Transport: remote.Transport})
	}
	skills := make([]PublicSkill, 0, len(record.Skills))
	for _, skill := range record.Skills {
		skills = append(skills, PublicSkill{ID: skill.ID, Title: skill.Title, URL: skill.URL, Status: skill.Status, SourceDigest: skill.SourceDigest})
	}
	var openapi *OpenAPI
	if record.OpenAPI != nil {
		copy := *record.OpenAPI
		openapi = &copy
	}
	listing, official, evidence := "", false, ""
	if record.Kind == "mcp" {
		listing = "https://gist.sire.run/library"
	}
	if record.Kind == "openapi" {
		official = record.Official
		evidence = record.OfficialEvidenceURL
	}
	return PublicRecord{
		Source:           RecordSource{ID: "gist", EntryID: record.ID, URL: "https://gist.sire.run/library"},
		ServerListingURL: listing, Official: official, OfficialEvidenceURL: evidence,
		ID:          record.ID,
		Kind:        record.Kind,
		ServiceID:   record.ServiceID,
		Title:       record.Title,
		Description: publicDescription(record),
		Version:     record.Version,
		Tags:        append([]string(nil), record.Tags...),
		Homepage:    record.Homepage,
		Repository:  record.Repository,
		Remotes:     remotes,
		Packages:    append([]Package(nil), record.Packages...),
		OpenAPI:     openapi,
		Auth:        Auth{Type: record.Auth.Type, CredentialNames: append([]string(nil), record.Auth.CredentialNames...), SetupURL: record.Auth.SetupURL},
		Skills:      skills,
	}
}

func PublicResultOf(result Result) PublicResult {
	items := make([]PublicRecord, 0, len(result.Items))
	for _, record := range result.Items {
		items = append(items, PublicRecordOf(record))
	}
	return PublicResult{Items: items, NextCursor: result.NextCursor, Total: result.Total, SnapshotRevision: result.SnapshotRevision}
}

func PublicExportOf(export Export) PublicExport {
	return PublicExport{Format: export.Format, Record: PublicRecordOf(export.Record)}
}

func publicDescription(record Record) string {
	description := strings.TrimSpace(record.Description)
	lower := strings.ToLower(description)
	for _, marker := range []string{
		"listed by make",
		"operator is not verified by gist",
		"review its account permissions",
		"this reachability check",
		"source listing",
		"workflow qualification",
	} {
		if strings.Contains(lower, marker) {
			if app, ok := publicAppForService(record.ServiceID); ok && strings.TrimSpace(app.Description) != "" {
				return app.Description
			}
			if len(record.Tags) > 0 {
				return fmt.Sprintf("%s integration for %s.", record.Title, strings.Join(record.Tags, ", "))
			}
			return fmt.Sprintf("%s integration.", record.Title)
		}
	}
	if description == "" {
		if record.Kind == "openapi" {
			return fmt.Sprintf("OpenAPI integration for %s. Use the published specification with a compatible client or adapter.", record.Title)
		}
		return fmt.Sprintf("MCP server for %s. Connect it from an MCP client to use this service.", record.Title)
	}
	return description
}
