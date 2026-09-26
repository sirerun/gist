package packages

import (
	"archive/zip"
	"bytes"
	"testing"
)

func zipBytes(t *testing.T, names ...string) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("x"))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestReadArchiveRejectsTraversal(t *testing.T) {
	if _, err := ReadArchive(zipBytes(t, "../escape"), DefaultLimits()); err == nil {
		t.Fatal("expected traversal rejection")
	}
}
func TestReadArchiveRejectsBackslash(t *testing.T) {
	if _, err := ReadArchive(zipBytes(t, `a\\b`), DefaultLimits()); err == nil {
		t.Fatal("expected backslash rejection")
	}
}
