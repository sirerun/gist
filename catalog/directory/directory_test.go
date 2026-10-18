package directory

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func fixture() Snapshot {
	return Snapshot{Format: SnapshotFormat, Revision: "test-v1", GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Sources: []Source{{ID: "test", URL: "https://example.com/source", Revision: "abc", SHA256: strings.Repeat("a", 64), RetrievedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Attribution: "test metadata"}}, Entries: []Record{{ID: "mcp:mail", Kind: "mcp", ServiceID: "mail", Title: "Mail Briefing", Description: "Read a mailbox", Tags: []string{"mail"}, Source: RecordSource{ID: "test", EntryID: "mail", URL: "https://example.com/mail"}, Remotes: []Remote{{URL: "https://example.com/mcp", Transport: "streamable-http"}}, Auth: Auth{Type: "oauth2"}}}}
}
func TestDefaultSnapshot(t *testing.T) {
	i, e := Default()
	if e != nil {
		t.Fatal(e)
	}
	if i.Count() != 13 {
		t.Fatalf("unexpected count %d", i.Count())
	}
	q, e := i.Search(Query{Tag: "zati-essentials", Limit: 100})
	if e != nil || q.Total != 4 {
		t.Fatalf("essentials %+v %v", q, e)
	}
	for _, r := range q.Items {
		if !r.Recommended || r.Featured || len(r.Verification) != 0 {
			t.Fatalf("imported qualification claim %s", r.ID)
		}
	}
	all, err := i.Search(Query{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	services := map[string]bool{}
	for _, r := range all.Items {
		if !r.Official || r.OfficialEvidenceURL == "" || services[r.ServiceID] || r.Kind != "mcp" || len(r.Remotes) == 0 {
			t.Fatalf("unadmitted public connector: %s", r.ID)
		}
		services[r.ServiceID] = true
	}
	for _, removed := range []string{"openapi:google-gmail", "openapi:google-calendar", "openapi:google-drive", "mcp:com.brave/brave-search"} {
		if _, ok := i.Get(removed); ok {
			t.Fatalf("retired route remains: %s", removed)
		}
	}
	data := DefaultSnapshot()
	data[0] = '!'
	if _, e = Default(); e != nil {
		t.Fatal("caller mutated embedded input")
	}
}
func TestIndexedSearchPagingAndImmutability(t *testing.T) {
	s := fixture()
	for n := 0; n < 57; n++ {
		r := s.Entries[0]
		r.ID = fmt.Sprintf("mcp:item-%02d", n)
		r.Title = fmt.Sprintf("Calendar Mail %02d", n)
		s.Entries = append(s.Entries, r)
	}
	i, e := New(s)
	if e != nil {
		t.Fatal(e)
	}
	s.Entries[0].Tags[0] = "corrupt"
	first, e := i.Search(Query{Text: "cal ma", Limit: 10})
	if e != nil || first.Total != 57 || len(first.Items) != 10 || first.NextCursor == "" {
		t.Fatalf("%+v %v", first, e)
	}
	next, e := i.Search(Query{Text: "cal ma", Limit: 10, Cursor: first.NextCursor})
	if e != nil || next.Items[0].ID == first.Items[0].ID {
		t.Fatalf("bad page %v", e)
	}
	if _, e = i.Search(Query{Text: "different", Cursor: first.NextCursor}); e == nil {
		t.Fatal("cursor query substitution accepted")
	}
	if _, e = i.Search(Query{Limit: 101}); e == nil {
		t.Fatal("unbounded query")
	}
	r, _ := i.Get("mcp:mail")
	r.Remotes[0].URL = "https://evil.example.com"
	r.Tags[0] = "mutated"
	r, _ = i.Get("mcp:mail")
	if r.Tags[0] != "mail" || r.Remotes[0].URL != "https://example.com/mcp" {
		t.Fatal("index mutated")
	}
}
func TestQualificationRequiresOperatorEvidence(t *testing.T) {
	r := fixture().Entries[0]
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	pin := "sha256:" + strings.Repeat("a", 64)
	r.Version = "1.2.3"
	r.Featured = true
	r.Recommended = true
	r.Official = true
	r.OfficialEvidenceURL = "https://example.com/vendor"
	if IsFeatured(r, now) {
		t.Fatal("upstream flags qualified")
	}
	r.FeaturedSelection = true
	r.Skills = []Skill{{ID: "brief", Title: "Brief inbox", URL: "https://example.com/skill", Status: "qualified", SourceDigest: pin}}
	r.Verification = []Qualification{{Client: "Zati", ClientVersion: "1.0.0", Workflow: "Read-only inbox briefing", Version: r.Version, ToolDigest: pin, SchemaDigest: pin, SkillDigest: pin, EvidenceURL: "https://example.com/receipt", CheckedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), Result: "passed"}}
	r.Verification[0].DescriptorDigest = DescriptorDigest(r)
	if !IsFeatured(r, now) {
		t.Fatal("complete evidence not eligible")
	}
	tests := []struct {
		name string
		edit func(*Record)
	}{{"remote changed", func(r *Record) { r.Remotes[0].URL = "https://replacement.example.com/mcp" }}, {"package changed", func(r *Record) { r.Packages = []Package{{Registry: "npm", Identifier: "replacement-server"}} }}, {"auth changed", func(r *Record) { r.Auth.Type = "bearer" }}, {"descriptor identity changed", func(r *Record) { r.ServiceID = "replacement" }}, {"missing descriptor pin", func(r *Record) { r.Verification[0].DescriptorDigest = "" }}, {"expired", func(r *Record) { r.Verification[0].ExpiresAt = now }}, {"future", func(r *Record) { r.Verification[0].CheckedAt = now.Add(time.Minute) }}, {"version", func(r *Record) { r.Verification[0].Version = "other" }}, {"missing schema", func(r *Record) { r.Verification[0].SchemaDigest = "" }}, {"different skill", func(r *Record) { r.Skills[0].SourceDigest = "sha256:" + strings.Repeat("b", 64) }}, {"unqualified skill", func(r *Record) { r.Skills[0].Status = "candidate" }}, {"missing client version", func(r *Record) { r.Verification[0].ClientVersion = "" }}, {"wrong client", func(r *Record) { r.Verification[0].Client = "fixture" }}, {"unselected", func(r *Record) { r.FeaturedSelection = false }}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copy := clone(r)
			tt.edit(&copy)
			if IsFeatured(copy, now) {
				t.Fatal("incomplete evidence eligible")
			}
		})
	}
}
func TestPublicPrivateMetadataAndNoCredentialURLs(t *testing.T) {
	for _, u := range []string{"http://localhost:8080/mcp", "https://127.0.0.1/mcp", "https://192.168.1.2/mcp", "https://machine.internal/mcp"} {
		r := fixture().Entries[0]
		r.Remotes[0].URL = u
		if ValidateRecord(r) == nil {
			t.Errorf("public accepted %s", u)
		}
		if e := ValidatePrivateRecord(r); e != nil {
			t.Errorf("private rejected %s: %v", u, e)
		}
	}
	for _, u := range []string{"https://user:secret@example.com/mcp", "https://example.com/mcp?token=secret", "https://example.com/mcp#secret"} {
		r := fixture().Entries[0]
		r.Remotes[0].URL = u
		if ValidateRecord(r) == nil || ValidatePrivateRecord(r) == nil {
			t.Errorf("credential-bearing URL accepted %s", u)
		}
	}
	r := fixture().Entries[0]
	r.Remotes = nil
	r.Packages = []Package{{Registry: "local", Identifier: "/Users/me/server"}}
	if ValidateRecord(r) == nil || ValidatePrivateRecord(r) != nil {
		t.Fatal("local package scope")
	}
}
func TestSnapshotRejectsUnknownDuplicateAndOversize(t *testing.T) {
	s := fixture()
	s.Entries = append(s.Entries, s.Entries[0])
	if _, e := New(s); e == nil {
		t.Fatal("duplicate accepted")
	}
	s = fixture()
	s.Entries[0].Source.ID = "missing"
	if _, e := New(s); e == nil {
		t.Fatal("unknown source accepted")
	}
	if _, e := Parse([]byte(`{"format":"gist.directory.snapshot/v1","unknown":1}`)); e == nil {
		t.Fatal("unknown field accepted")
	}
	if _, e := Parse(bytes.Repeat([]byte{' '}, MaxSnapshotBytes+1)); e == nil {
		t.Fatal("oversize snapshot accepted")
	}
}
func TestConcurrentReadIsIndependent(t *testing.T) {
	i, e := New(fixture())
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				r, _ := i.Get("mcp:mail")
				r.Tags[0] = "changed"
				if _, e := i.Search(Query{Text: "mail"}); e != nil {
					t.Error(e)
				}
			}
		}()
	}
	wg.Wait()
}
func TestIndexedScale100000(t *testing.T) {
	if testing.Short() {
		t.Skip("scale")
	}
	s := fixture()
	r := s.Entries[0]
	s.Entries = nil
	for n := 0; n < 100000; n++ {
		v := clone(r)
		v.ID = fmt.Sprintf("mcp:service-%06d", n)
		v.ServiceID = v.ID
		v.Title = fmt.Sprintf("Service %06d", n)
		s.Entries = append(s.Entries, v)
	}
	i, e := New(s)
	if e != nil {
		t.Fatal(e)
	}
	result, e := i.Search(Query{Text: "099999", Limit: 10})
	if e != nil || result.Total != 1 {
		t.Fatalf("scale query %v total %d", e, result.Total)
	}
}

func TestPackageDescriptorsAreNotCommands(t *testing.T) {
	for _, registry := range []string{"npm", "pypi", "oci", "local"} {
		for _, identifier := range []string{"server;echo", "server&&echo", "server|echo", "$(command)", "`command`", "server>file", "server<file", "server'quoted", "server\"quoted", "server*", "server?", "server!"} {
			r := fixture().Entries[0]
			r.Remotes = nil
			r.Packages = []Package{{Registry: registry, Identifier: identifier}}
			if ValidatePrivateRecord(r) == nil {
				t.Errorf("accepted command-like %s package", registry)
			}
		}
	}
}
