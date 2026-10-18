package semantic

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

const snapshotFormat = "gist.public-semantic-snapshot/v1"
const maxSnapshotBytes = 128 << 20

type snapshotArtifact struct {
	Format       string        `json:"format"`
	ModelVersion string        `json:"model_version"`
	CorpusDigest string        `json:"corpus_digest"`
	Generation   string        `json:"generation"`
	Rows         []snapshotRow `json:"rows"`
}
type snapshotRow struct {
	ID         string    `json:"id"`
	TextSHA256 string    `json:"text_sha256"`
	Vector     []float64 `json:"vector"`
}

func snapshotGeneration(model, revision string, docs []document) string {
	hash := sha256.New()
	fmt.Fprintf(hash, "public-hybrid-v1:floor0.30:rrf60:cap32:%s\n%s", model, revision)
	for _, d := range docs {
		fmt.Fprintf(hash, "%d:%s%d:%s", len(d.id), d.id, len(d.text), d.text)
	}
	return hex.EncodeToString(hash.Sum(nil))
}
func corpusDigest(revision string, docs []document) string {
	hash := sha256.New()
	fmt.Fprintf(hash, "gist.public-semantic-corpus/v1:%d:%s", len(revision), revision)
	for _, d := range docs {
		fmt.Fprintf(hash, "%d:%s%d:%s", len(d.id), d.id, len(d.text), d.text)
	}
	return hex.EncodeToString(hash.Sum(nil))
}
func textDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// WriteSnapshot exports the complete warmed generation. The caller owns
// compression and storage; the artifact contains only public corpus vectors.
func (s *Service) WriteSnapshot(w io.Writer) error {
	snap := s.ready.Load()
	if snap == nil || s.embedder == nil {
		return errors.New("semantic: snapshot unavailable")
	}
	artifact := snapshotArtifact{Format: snapshotFormat, ModelVersion: s.embedder.ModelVersion(), CorpusDigest: corpusDigest(s.index.Revision(), snap.docs), Generation: snap.version, Rows: make([]snapshotRow, len(snap.docs))}
	for i, d := range snap.docs {
		artifact.Rows[i] = snapshotRow{ID: d.id, TextSHA256: textDigest(d.text), Vector: snap.vectors[i]}
	}
	// Bound the exported form as well as the loader's input.
	body, err := json.Marshal(artifact)
	if err != nil || len(body)+1 > maxSnapshotBytes {
		return errors.New("semantic: snapshot encoding exceeds bounds")
	}
	body = append(body, '\n')
	if n, writeErr := w.Write(body); writeErr != nil || n != len(body) {
		return errors.New("semantic: snapshot write failed")
	}
	return nil
}

// LoadSnapshot accepts only an exact pinned runtime/corpus generation. Failure
// never publishes a partially validated snapshot or starts model inference.
func (s *Service) LoadSnapshot(r io.Reader) error {
	if s.embedder == nil {
		return errors.New("semantic: no embedder")
	}
	select {
	case s.slot <- struct{}{}:
		defer func() { <-s.slot }()
	default:
		return errors.New("semantic: snapshot busy")
	}
	body, err := io.ReadAll(io.LimitReader(r, maxSnapshotBytes+1))
	if err != nil || len(body) > maxSnapshotBytes {
		return errors.New("semantic: snapshot read exceeds bounds")
	}
	// Reject duplicate keys, null scalars, and noncanonical JSON before typed
	// decoding can silently coerce null vector components into numeric zeros.
	strict := json.NewDecoder(bytes.NewReader(body))
	if err = checkSnapshotJSON(strict, 0); err != nil {
		return errors.New("semantic: ambiguous snapshot schema")
	}
	if _, err = strict.Token(); err != io.EOF {
		return errors.New("semantic: trailing snapshot bytes")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var artifact snapshotArtifact
	if err = decoder.Decode(&artifact); err != nil {
		return errors.New("semantic: invalid snapshot schema")
	}
	docs, err := s.documents()
	if err != nil {
		return err
	}
	model := s.embedder.ModelVersion()
	expected := snapshotGeneration(model, s.index.Revision(), docs)
	if artifact.Format != snapshotFormat || artifact.ModelVersion != model || artifact.CorpusDigest != corpusDigest(s.index.Revision(), docs) || artifact.Generation != expected || len(artifact.Rows) != len(docs) {
		return errors.New("semantic: snapshot identity/corpus mismatch")
	}
	snap := &snapshot{docs: docs, version: expected, vectors: make([][]float64, len(docs))}
	for i, row := range artifact.Rows {
		if row.ID != docs[i].id || row.TextSHA256 != textDigest(docs[i].text) || !valid(row.Vector) {
			return errors.New("semantic: invalid snapshot document/vector")
		}
		snap.vectors[i] = row.Vector
	}
	s.ready.Store(snap)
	return nil
}

func checkSnapshotJSON(d *json.Decoder, depth int) error {
	if depth > 8 {
		return errors.New("snapshot depth")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	if token == nil {
		return errors.New("null snapshot value")
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		keys := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || keys[name] {
				return errors.New("duplicate snapshot field")
			}
			switch name {
			case "format", "model_version", "corpus_digest", "generation", "rows", "id", "text_sha256", "vector":
			default:
				return errors.New("noncanonical snapshot field")
			}
			keys[name] = true
			if err = checkSnapshotJSON(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := checkSnapshotJSON(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid snapshot delimiter")
	}
	_, err = d.Token()
	return err
}

// LoadSnapshotFile loads a deployment-owned gzip artifact. Reading the complete
// bounded stream verifies gzip trailers/checksums before snapshot publication.
func (s *Service) LoadSnapshotFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return errors.New("semantic: snapshot file unavailable")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 32<<20 {
		return errors.New("semantic: invalid snapshot file bounds")
	}
	limited := &io.LimitedReader{R: file, N: (32 << 20) + 1}
	compressed, err := gzip.NewReader(limited)
	if err != nil {
		return errors.New("semantic: invalid compressed snapshot")
	}
	defer compressed.Close()
	// Buffer before LoadSnapshot: compressed-size failure cannot publish ready.
	body, err := io.ReadAll(io.LimitReader(compressed, maxSnapshotBytes+1))
	if err != nil || len(body) > maxSnapshotBytes || limited.N <= 0 {
		return errors.New("semantic: compressed snapshot failed integrity/bounds")
	}
	return s.LoadSnapshot(bytes.NewReader(body))
}
