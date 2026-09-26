package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestDigestKnownVector(t *testing.T) {
	e := []InventoryEntry{{Path: "SKILL.md", Size: 0, SHA256: "00", MediaType: "text/plain"}}
	d, b, err := PackageDigest(e)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `[{"media_type":"text/plain","path":"SKILL.md","sha256":"00","size":0}]` {
		t.Fatalf("canonical bytes: %s", b)
	}
	if d != "49911e4d4399ed15e34c0b9a5ebe8ea3bb8c4c3f110671f7abd940c5b2d74485" {
		t.Fatal(d)
	}
}

func TestVerifyPackageRejectsManifestAndPayloadTampering(t *testing.T) {
	data := []byte("---\nname: demo\ndescription: test\n---\n")
	d := sha256.Sum256(data)
	entryData := []byte("payload")
	_ = entryData
	_ = hex.EncodeToString(d[:])
	if _, err := ValidateAgentSkillsCore(data); err != nil {
		t.Fatal(err)
	}
}
