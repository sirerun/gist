package packages

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"github.com/sirerun/gist/catalog/integrity"
	"github.com/sirerun/gist/catalog/ports"
	"testing"
)

func TestRetrieveSkillVerifiesExternalPinsAndAssetClosure(t *testing.T) {
	files := map[string][]byte{"SKILL.md": []byte("---\nname: inbox\ndescription: Brief inbox\n---\nUse assets/template.txt"), "assets/template.txt": []byte("Unread messages")}
	inventory := []integrity.InventoryEntry{}
	for name, data := range files {
		inventory = append(inventory, integrity.InventoryEntry{Path: name, Size: int64(len(data)), SHA256: digestBytes(data), MediaType: "text/plain"})
	}
	digest, _, err := integrity.PackageDigest(inventory)
	if err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{ID: "inbox", Version: "1", Inventory: inventory, Trust: "operator_asserted"}
	manifest.PackageDigest.Algorithm = "sha256"
	manifest.PackageDigest.Value = digest
	manifest.Publication.Status = "approved"
	mb, _ := json.Marshal(manifest)
	files["manifest.json"] = mb
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	ref := ports.ArtifactRef{WorkspaceID: "w", Kind: ports.KindSkill, ID: "inbox", Version: "1"}
	pin := ports.ArtifactPin{Ref: ref, Digest: ports.Digest{Algorithm: "sha256", Value: digestBytes(buf.Bytes())}, ManifestDigest: ports.Digest{Algorithm: "sha256", Value: digestBytes(mb)}}
	record := ports.CatalogRecord{Ref: ref, State: "published", Digest: pin.Digest, ManifestDigest: pin.ManifestDigest}
	principal := ports.Principal{Subject: "p", WorkspaceID: "w"}
	for _, tc := range []struct {
		name   string
		mutate func(*Retriever, *ports.ArtifactPin)
	}{
		{"valid", func(*Retriever, *ports.ArtifactPin) {}},
		{"wrong trusted digest", func(_ *Retriever, p *ports.ArtifactPin) { p.Digest.Value = digestBytes([]byte("other")) }},
		{"wrong workspace", func(_ *Retriever, p *ports.ArtifactPin) { p.Ref.WorkspaceID = "other" }},
		{"tampered bytes", func(r *Retriever, _ *ports.ArtifactPin) {
			r.Artifacts = artifactDouble{value: buf.String() + "changed"}
		}},
		{"revoked", func(r *Retriever, _ *ports.ArtifactPin) {
			rr := record
			rr.State = "revoked"
			r.Catalog = catalogDouble{record: rr}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Retriever{Authorizer: &authDouble{allowed: true}, Catalog: catalogDouble{record: record}, Artifacts: artifactDouble{value: buf.String()}}
			expected := pin
			tc.mutate(&r, &expected)
			pkg, err := r.RetrieveSkill(context.Background(), principal, expected, DefaultLimits())
			if tc.name == "valid" {
				if err != nil {
					t.Fatal(err)
				}
				if string(pkg.Files["assets/template.txt"]) != "Unread messages" {
					t.Fatal("asset lost")
				}
			} else if err == nil {
				t.Fatal("unsafe package accepted")
			}
		})
	}
}
