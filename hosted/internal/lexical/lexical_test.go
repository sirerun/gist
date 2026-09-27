package lexical

import (
	"reflect"
	"testing"
)

func TestTermsTokenizeStemAndDropStopwords(t *testing.T) {
	got := Terms("Parsing the documents, via gist/document/parse@1.0.0 capabilities")
	want := []string{"pars", "document", "gist", "document", "pars", "1", "0", "0", "capability"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Terms = %q, want %q", got, want)
	}
}

func TestStemConflatesRegistryInflections(t *testing.T) {
	for _, group := range [][]string{
		{"parse", "parser", "parsing", "parses"},
		{"create", "creates"},
		{"capability", "capabilities"},
		{"message", "messages"},
	} {
		first := Stem(group[0])
		for _, w := range group[1:] {
			if Stem(w) != first {
				t.Errorf("Stem(%q)=%q, want %q (same as %q)", w, Stem(w), first, group[0])
			}
		}
	}
	if Stem("class") != "class" || Stem("status") != "status" {
		t.Fatal("stemming must not strip the s of -ss or -us words")
	}
}

func TestScoreWeightsFieldsAndRejectsNoMatch(t *testing.T) {
	capability := Document{ID: "gist/document/parse", LogicalName: "document parse", Kind: "capability", Taxonomy: []string{"document", "read_only"}, Description: "Parse a document into text."}
	skill := Document{ID: "gist/skill/asset-skill", LogicalName: "asset-skill", Kind: "skill", Capabilities: []string{"gist/document/parse"}, Taxonomy: []string{"document"}, Description: "Offline document parsing fixture."}

	if m := Score("document parse capability", capability); m != (Match{Terms: 3, Weight: 3 + 3 + 2}) {
		t.Fatalf("capability match = %+v", m)
	}
	// "document" and "parse" hit the skill only through its capability ID,
	// taxonomy and prose, so it ranks below the capability named for them.
	if m := Score("document parse capability", skill); m.Better(Score("document parse capability", capability)) >= 0 {
		t.Fatalf("skill %+v must rank below the named capability", m)
	}
	if m := Score("weather forecast lookup", capability); m.Matched() {
		t.Fatalf("unrelated query matched: %+v", m)
	}
	if m := Score("the of and", capability); m.Matched() {
		t.Fatalf("stopword-only query matched: %+v", m)
	}
}

func TestDocumentFromMetadataUnpinsCapabilities(t *testing.T) {
	d, err := DocumentFromMetadata("binding", "gist/x/bind", []byte(`{"logical_name":"bind","summary":"s","tags":["t"],"taxonomy":["communication.message"],"required_capabilities":["gist/communication/message.send@1.0.0"],"capability_ref":"gist/communication/message.send@1.0.0"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.Capabilities, []string{"gist/communication/message.send", "gist/communication/message.send"}) {
		t.Fatalf("capabilities = %q", d.Capabilities)
	}
	if !reflect.DeepEqual(d.Taxonomy, []string{"t", "communication.message"}) {
		t.Fatalf("taxonomy = %q", d.Taxonomy)
	}
	if _, err := DocumentFromMetadata("skill", "x", []byte(`{`)); err == nil {
		t.Fatal("malformed metadata must be reported")
	}
}

func TestMatchBetterIsATotalPreorder(t *testing.T) {
	a, b, c := Match{Terms: 2, Weight: 3}, Match{Terms: 2, Weight: 5}, Match{Terms: 1, Weight: 9}
	if b.Better(a) != 1 || a.Better(c) != 1 || a.Better(a) != 0 || c.Better(b) != -1 {
		t.Fatal("more matched terms outrank weight; weight breaks term ties")
	}
}
