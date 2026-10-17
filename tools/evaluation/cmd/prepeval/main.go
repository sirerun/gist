// prepeval exercises public preparation on a fixed shared synthetic catalog.
// It is a source smoke runner, not a held-out or whole-task performance study.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sirerun/gist/catalog/ports"
	"github.com/sirerun/gist/catalog/preparation"
	"github.com/sirerun/gist/tools/evaluation/fixture"
)

type input struct {
	ID      string `json:"id"`
	Family  string `json:"family"`
	Stratum string `json:"stratum"`
	Intent  string `json:"intent"`
	Fixture struct {
		Status string `json:"status"`
	} `json:"fixture"`
}
type vectorEntry struct {
	Purpose string    `json:"purpose"`
	Text    string    `json:"text"`
	Vector  []float64 `json:"vector"`
}
type vectors struct {
	ModelVersion string        `json:"model_version"`
	Entries      []vectorEntry `json:"entries"`
}

func (v vectors) Embed(_ context.Context, request preparation.EmbeddingRequest) (preparation.EmbeddingResult, error) {
	if request.ModelVersion != v.ModelVersion {
		return preparation.EmbeddingResult{}, errors.New("stale model/index version")
	}
	result := preparation.EmbeddingResult{ModelVersion: v.ModelVersion}
	for _, text := range request.Texts {
		found := false
		for _, entry := range v.Entries {
			if entry.Purpose == request.Purpose && entry.Text == text {
				result.Vectors = append(result.Vectors, append([]float64(nil), entry.Vector...))
				found = true
				break
			}
		}
		if !found {
			return preparation.EmbeddingResult{}, errors.New("vector absent for exact current authorized text")
		}
	}
	return result, nil
}

type attempt struct {
	Schema         string         `json:"schema"`
	ID             string         `json:"id"`
	Family         string         `json:"family"`
	Stratum        string         `json:"stratum"`
	Arm            string         `json:"arm"`
	Repeat         int            `json:"repeat"`
	Status         string         `json:"status"`
	Error          string         `json:"error,omitempty"`
	Source         string         `json:"source"`
	CatalogVersion string         `json:"catalog_version"`
	RecipeVersion  string         `json:"recipe_version"`
	ModelVersion   string         `json:"model_version,omitempty"`
	ElapsedSeconds float64        `json:"elapsed_seconds"`
	ContextBytes   int            `json:"context_bytes"`
	Selected       string         `json:"selected,omitempty"`
	Kind           string         `json:"kind"`
	Usage          map[string]any `json:"usage"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	inputs := flag.String("inputs", "", "synthetic development input-only JSONL; never gold labels")
	vectorFile := flag.String("vectors", "", "pinned local adapter vector artifact")
	dry := flag.Bool("dry-run", false, "no inference/provider calls; estimate source attempts")
	docs := flag.Bool("embedding-inputs", false, "emit fixed catalog descriptions and actual query text for local adapter")
	source := flag.String("source", "", "exact tracked source SHA")
	flag.Parse()
	if *inputs == "" || *source == "" {
		return errors.New("--inputs and --source are required")
	}
	file, err := os.Open(*inputs)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	cases := []input{}
	for scanner.Scan() {
		var item input
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			return err
		}
		if item.ID == "" || item.Family == "" || item.Intent == "" {
			return errors.New("invalid synthetic input")
		}
		cases = append(cases, item)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	if *dry {
		return enc.Encode(map[string]any{"schema": "gist.preparation.dry-run/v1", "cases": len(cases), "source_attempts": len(cases) * 5, "unavailable_provider_attempts": len(cases) * 2, "provider_calls": 0, "paid_model_calls": 0, "maximum_adapter_lookup_calls": len(cases) * 6, "token_estimate": nil, "dollar_estimate": nil, "reason": "Source smoke uses existing local vectors; whole-task/provider comparisons are NOT_RUN; no token/rate oracle", "context_budget_bytes": 65536})
	}
	if *docs {
		c, e := fixture.New("ready")
		if e != nil {
			return e
		}
		page, e := c.Search(context.Background(), structSearch())
		if e != nil {
			return e
		}
		texts := []string{}
		for _, record := range page.Records {
			var meta struct {
				LogicalName string `json:"logical_name"`
				Summary     string `json:"summary"`
			}
			if e := json.Unmarshal(record.Metadata, &meta); e != nil {
				return e
			}
			texts = append(texts, meta.LogicalName+" "+meta.Summary)
		}
		if e := enc.Encode(map[string]any{"purpose": "document", "texts": texts}); e != nil {
			return e
		}
		for _, item := range cases {
			if e := enc.Encode(map[string]any{"purpose": "query", "texts": []string{item.Intent}}); e != nil {
				return e
			}
		}
		return nil
	}
	var vec *vectors
	if *vectorFile != "" {
		raw, e := os.ReadFile(*vectorFile)
		if e != nil {
			return e
		}
		var value vectors
		if e := json.Unmarshal(raw, &value); e != nil {
			return e
		}
		if value.ModelVersion == "" {
			return errors.New("unversioned vector artifact")
		}
		vec = &value
	}
	for _, item := range cases {
		for _, arm := range []string{"L0", "L1", "E0", "H0", "H1", "C0", "C1"} {
			row := attempt{Schema: "gist.preparation.attempt/v1", ID: item.ID, Family: item.Family, Stratum: item.Stratum, Arm: arm, Repeat: 1, Source: *source, CatalogVersion: fixture.CatalogVersion, RecipeVersion: preparation.RecipeVersion, Kind: "controlled_source_smoke_not_retrieval_or_whole_task_benchmark", Usage: map[string]any{"provider_calls": 0, "paid_model_calls": 0, "main_model_tokens": nil, "cached_tokens": nil, "embedding_tokens": nil, "dollars": nil, "embedding_execution": "precomputed real pinned local vectors; separately qualified; lookup latency excludes inference"}}
			if arm == "C0" || arm == "C1" {
				row.Status = "NOT_RUN"
				row.Error = "No admitted provider/model/account/call-dollar-expiry manifest; C0 native discovery and C1 preregistered direct tools remain distinct"
				if err := enc.Encode(row); err != nil {
					return err
				}
				continue
			}
			c, e := fixture.New(item.Fixture.Status)
			if e != nil {
				row.Status = "NOT_RUN"
				row.Error = e.Error()
				if err := enc.Encode(row); err != nil {
					return err
				}
				continue
			}
			service, e := c.Service()
			if e != nil {
				return e
			}
			request := fixture.Request(item.Intent)
			request.Guide = arm == "L1" || arm == "H1"
			switch arm {
			case "E0":
				request.Mode = preparation.Semantic
			case "H0", "H1":
				request.Mode = preparation.Hybrid
			}
			if request.Mode != preparation.Lexical {
				if vec == nil {
					row.Status = "NOT_RUN"
					row.Error = "pinned real local vectors unavailable"
					if err := enc.Encode(row); err != nil {
						return err
					}
					continue
				}
				service.Embedder = *vec
				service.ModelVersion = vec.ModelVersion
				row.ModelVersion = vec.ModelVersion
			}
			before := time.Now()
			result, e := service.Prepare(context.Background(), request)
			row.ElapsedSeconds = time.Since(before).Seconds()
			row.Status = result.Status
			if e != nil {
				row.Status = "ERROR"
				row.Error = e.Error()
			}
			if result.Selected != nil {
				row.Selected = result.Provenance.SkillPin.Ref.ID + "@" + result.Provenance.SkillPin.Ref.Version
				bytes, e := json.Marshal(result)
				if e != nil {
					return e
				}
				row.ContextBytes = len(bytes)
			}
			if err := enc.Encode(row); err != nil {
				return err
			}
		}
	}
	return nil
}

func structSearch() ports.SearchQuery {
	return ports.SearchQuery{Principal: fixture.Principal, Kinds: []ports.ArtifactKind{ports.KindSkill}, Limit: 100, MaxBytes: 65536}
}
