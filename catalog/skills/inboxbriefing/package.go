// Package inboxbriefing provides the maintained read-only inbox skill and its
// template. Importing this package neither admits a skill nor grants mailbox
// access. The consumer supplies trust policy, authorization and provider bindings.
package inboxbriefing

import (
	"archive/zip"
	"bytes"
	"embed"
	"io/fs"
	"sort"
)

const SkillID = "zati/inbox-briefing"
const Version = "1.0.0"
const CapabilityID = "mail/inbox/read"
const CapabilityVersion = "1.0.0"
const TemplatePath = "assets/briefing-template.md"

//go:embed SKILL.md manifest.json assets/briefing-template.md
var source embed.FS

// Files returns independently owned copies of the maintained package files.
// The manifest declares operator-asserted provenance, not a verified signature.
func Files() (map[string][]byte, error) {
	files := make(map[string][]byte)
	err := fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := source.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = data
		return nil
	})
	return files, err
}

// Archive returns a deterministic ZIP retaining every declared asset. This is
// inert content; callers must verify its pins and admit it under their policy.
func Archive() ([]byte, error) {
	files, err := Files()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
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
