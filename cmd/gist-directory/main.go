// gist-directory inspects pinned directory metadata without connecting servers.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/sirerun/gist/catalog/directory"
)

func main() {
	path := flag.String("snapshot", "", "optional snapshot file (defaults to embedded snapshot)")
	text := flag.String("query", "", "search text")
	kind := flag.String("kind", "", "mcp or openapi")
	tag := flag.String("tag", "", "tag filter")
	id := flag.String("id", "", "exact record ID to export")
	cursor := flag.String("cursor", "", "opaque search cursor")
	limit := flag.Int("limit", 24, "maximum results, 1–100")
	validate := flag.Bool("validate", false, "validate and report snapshot metadata")
	flag.Parse()
	data := directory.DefaultSnapshot()
	var err error
	if *path != "" {
		data, err = os.ReadFile(*path)
		if err != nil {
			fatal(err)
		}
	}
	index, err := directory.Parse(data)
	if err != nil {
		fatal(err)
	}
	var out any
	if *validate {
		out = struct {
			Revision string             `json:"revision"`
			Count    int                `json:"count"`
			Sources  []directory.Source `json:"sources"`
		}{index.Revision(), index.Count(), index.Sources()}
	} else if *id != "" {
		record, ok := index.Get(*id)
		if !ok {
			fatal(fmt.Errorf("record not found"))
		}
		out = directory.Export{Format: directory.ExportFormat, Record: record}
	} else {
		out, err = index.Search(directory.Query{Text: *text, Kind: *kind, Tag: *tag, Cursor: *cursor, Limit: *limit})
		if err != nil {
			fatal(err)
		}
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(out); err != nil {
		fatal(err)
	}
}
func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
