// Package essentials retains complete inert screening candidates for five
// read-only workflows. It neither admits skills nor implements provider bindings.
package essentials

import (
	"archive/zip"
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

const Version = "0.1.0"
const DraftContractVersion = "0.1.0"

type Workflow struct{ Key, SkillID, Title string }

// List returns the maintained screening candidates, not installed capabilities.
func List() []Workflow {
	return []Workflow{
		{"inbox-briefing", "gist/essentials/inbox-briefing", "Read-only inbox briefing"},
		{"calendar-briefing", "gist/essentials/calendar-briefing", "Read-only calendar briefing"},
		{"file-discovery", "gist/essentials/file-discovery", "Read-only file discovery"},
		{"github-pr-review", "gist/essentials/github-pr-review", "Read-only public GitHub pull request review"},
		{"web-research", "gist/essentials/web-research", "Read-only web research"},
	}
}

//go:embed sources/*/SKILL.md sources/*/manifest.json sources/*/assets/*
var source embed.FS

// Files returns independently owned bytes for one complete screening package.
// Draft capability references must be designed and mapped by a client before use.
func Files(key string) (map[string][]byte, error) {
	valid := false
	for _, w := range List() {
		if key == w.Key {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("essentials: unknown workflow %q", key)
	}
	sub, err := fs.Sub(source, "sources/"+key)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	err = fs.WalkDir(sub, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(sub, path)
		if err != nil {
			return err
		}
		files[path] = append([]byte(nil), data...)
		return nil
	})
	return files, err
}

// Archive deterministically retains the manifest, instructions and every asset.
// These bytes are candidates for review, not trusted or executable approval.
func Archive(key string) ([]byte, error) {
	files, err := Files(key)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		header.SetMode(0644)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(files[name]); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
