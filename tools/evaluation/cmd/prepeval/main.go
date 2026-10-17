// prepeval exercises public preparation on a fixed shared synthetic catalog.
// It is a source smoke runner, not a held-out or whole-task performance study.
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/sirerun/gist/catalog/ports"
	"github.com/sirerun/gist/catalog/preparation"
	"github.com/sirerun/gist/tools/evaluation/fixture"
)

type input struct {
	Split   string `json:"split"`
	ID      string `json:"id"`
	Family  string `json:"family"`
	Stratum string `json:"stratum"`
	Intent  string `json:"intent"`
	Fixture struct {
		SkillID          string          `json:"skill_id"`
		SkillVersion     string          `json:"skill_version"`
		ToolAlternatives [][]string      `json:"tool_alternatives"`
		Dependencies     []string        `json:"dependencies"`
		Status           string          `json:"status"`
		Transition       json.RawMessage `json:"transition"`
		Scenario         json.RawMessage `json:"scenario"`
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
	PolicyVersion    string         `json:"policy_version"`
	TopK             int            `json:"top_k"`
	SemanticFloor    float64        `json:"semantic_floor"`
	RRFConstant      int            `json:"rrf_constant"`
	CatalogSHA256    string         `json:"catalog_sha256"`
	ComparisonStatus string         `json:"comparison_status"`
	ComparisonReason string         `json:"comparison_reason,omitempty"`
	AttemptIndex     int            `json:"attempt_index"`
	RunKind          string         `json:"run_kind"`
	Phase            string         `json:"phase"`
	InputsSHA256     string         `json:"inputs_sha256"`
	Capabilities     []string       `json:"capabilities,omitempty"`
	Tools            []string       `json:"tools,omitempty"`
	CompleteContext  bool           `json:"complete_context"`
	ResultSHA256     string         `json:"result_sha256,omitempty"`
	Schema           string         `json:"schema"`
	ID               string         `json:"id"`
	Family           string         `json:"family"`
	Stratum          string         `json:"stratum"`
	Arm              string         `json:"arm"`
	Repeat           int            `json:"repeat"`
	Status           string         `json:"status"`
	Error            string         `json:"error,omitempty"`
	Source           string         `json:"source"`
	CatalogVersion   string         `json:"catalog_version"`
	RecipeVersion    string         `json:"recipe_version"`
	ModelVersion     string         `json:"model_version,omitempty"`
	ElapsedSeconds   float64        `json:"elapsed_seconds"`
	ContextBytes     int            `json:"context_bytes"`
	Selected         string         `json:"selected,omitempty"`
	Kind             string         `json:"kind"`
	Usage            map[string]any `json:"usage"`
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
	attemptIndex := flag.Int("attempt-index", 1, "explicit retry/attempt identity, never hidden replacement")
	runKind := flag.String("run-kind", "source_cold", "source_cold, source_warm or recovery; does not describe inference cache")
	phase := flag.String("phase", "development", "development or heldout; heldout requires frozen admission")
	admissionFile := flag.String("admission", "", "frozen run manifest for heldout admission")
	catalogFile := flag.String("catalog", "", "explicit frozen catalog manifest; no query/gold-derived inventory")
	catalogSHA := flag.String("catalog-sha256", "", "required exact content SHA for external manifest")
	exportCatalog := flag.Bool("export-catalog", false, "export default fixed shared catalog for review/roundtrip")
	policyFile := flag.String("task-policy", "", "independently reviewed evaluation fixture policy, never live authority")
	policySHA := flag.String("task-policy-sha256", "", "required exact policy content SHA256")
	exportPolicy := flag.Bool("export-task-policy-template", false, "export maintained pins without admitting any input query")
	topK := flag.Int("top-k", 4, "development candidate count: 4, 8 or 16; frozen for heldout")
	flag.Parse()
	if *topK != 4 && *topK != 8 && *topK != 16 {
		return errors.New("unsupported top-k; require 4, 8 or 16")
	}
	if *exportPolicy {
		p, err := policyTemplate()
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(p)
	}
	var policy *taskPolicy
	policyVersion := defaultPolicyIdentity
	if *policyFile != "" {
		var err error
		policy, err = loadPolicy(*policyFile, *policySHA)
		if err != nil {
			return err
		}
		policyVersion = policy.identity
	} else if *policySHA != "" {
		return errors.New("task-policy-sha256 requires task-policy")
	}
	if *attemptIndex <= 0 || (*runKind != "source_cold" && *runKind != "source_warm" && *runKind != "recovery") {
		return errors.New("invalid explicit attempt/run kind")
	}
	sourceBytes, sourceErr := hex.DecodeString(*source)
	if sourceErr != nil || (len(sourceBytes) != 20 && len(sourceBytes) != 32) || hex.EncodeToString(sourceBytes) != *source {
		return errors.New("exact immutable lowercase source SHA required")
	}
	if *inputs == "" || *source == "" {
		return errors.New("--inputs and --source are required")
	}
	if *phase != "development" && *phase != "heldout" {
		return errors.New("invalid evaluation phase")
	}
	base, err := fixture.New("ready")
	if err != nil {
		return err
	}
	rawCatalog, err := fixture.Export(base)
	if err != nil {
		return err
	}
	actualCatalogSHA := fixture.Hash(rawCatalog).Value
	if *catalogFile != "" {
		if !digestString(*catalogSHA) {
			return errors.New("external catalog requires exact SHA256")
		}
		base, err = fixture.LoadManifest(*catalogFile, *catalogSHA)
		if err != nil {
			return err
		}
		actualCatalogSHA = *catalogSHA
	}
	if policy != nil && policy.CatalogSHA256 != actualCatalogSHA {
		return errors.New("task policy catalog SHA mismatch")
	}
	catalogVersion := "sha256:" + actualCatalogSHA
	if *exportCatalog {
		if *catalogFile != "" {
			return errors.New("export only applies to default fixed catalog")
		}
		_, err := os.Stdout.Write(rawCatalog)
		return err
	}
	admission := map[string]any{}
	if *phase == "heldout" {
		if *admissionFile == "" {
			return errors.New("heldout refused before source/configuration freeze admission")
		}
		raw, e := os.ReadFile(*admissionFile)
		if e != nil {
			return e
		}
		if e = json.Unmarshal(raw, &admission); e != nil {
			return e
		}
		admittedModel, modelOK := admission["model_version"].(string)
		if !modelOK || admittedModel == "" {
			return errors.New("heldout requires frozen model/runtime version")
		}
		if admission["candidate_frozen"] != true || admission["model_version"] == "" || admission["candidate_source"] != *source || admission["catalog_version"] != catalogVersion || admission["catalog_sha256"] != actualCatalogSHA || admission["recipe_version"] != preparation.RecipeVersion || admission["semantic_floor"] != float64(0.30) || admission["top_k"] != float64(*topK) || admission["policy_version"] != policyVersion || admission["rrf_constant"] != float64(60) || !digestString(admission["development_selection_sha256"]) || !digestString(admission["inputs_sha256"]) || !digestString(admission["labels_sha256"]) {
			return errors.New("heldout source/configuration/selection manifest mismatch")
		}
	}
	inputBytes, e := readBounded(*inputs, 16<<20)
	if e != nil {
		return e
	}
	sum := sha256.Sum256(inputBytes)
	inputHash := hex.EncodeToString(sum[:])
	if *phase == "heldout" && admission["inputs_sha256"] != inputHash {
		return errors.New("heldout input hash differs from frozen manifest")
	}
	scanner := bufio.NewScanner(strings.NewReader(string(inputBytes)))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	cases := []input{}
	for scanner.Scan() {
		var item input
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			return err
		}
		if item.Split != *phase || item.ID == "" || item.Family == "" || item.Intent == "" {
			return errors.New("invalid synthetic input")
		}
		cases = append(cases, item)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	if *dry {
		return enc.Encode(map[string]any{"schema": "gist.preparation.dry-run/v1", "policy_version": policyVersion, "top_k": *topK, "cases": len(cases), "source_attempts": len(cases) * 5, "unavailable_provider_attempts": len(cases) * 2, "provider_calls": 0, "paid_model_calls": 0, "maximum_adapter_lookup_calls": len(cases) * 6, "token_estimate": nil, "dollar_estimate": nil, "reason": "Source smoke uses existing local vectors; whole-task/provider comparisons are NOT_RUN; no token/rate oracle", "context_budget_bytes": 65536})
	}
	if *docs {
		c := fixture.Clone(base)
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
		raw, e := readBounded(*vectorFile, 64<<20)
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
	if *phase == "heldout" && (vec == nil || admission["model_version"] != vec.ModelVersion) {
		return errors.New("heldout model/runtime/index mismatch")
	}
	for _, item := range cases {
		for _, arm := range []string{"L0", "L1", "E0", "H0", "H1", "C0", "C1"} {
			row := attempt{PolicyVersion: policyVersion, TopK: *topK, SemanticFloor: 0.30, RRFConstant: 60, AttemptIndex: *attemptIndex, RunKind: *runKind, Phase: *phase, InputsSHA256: inputHash, Schema: "gist.preparation.attempt/v1", ID: item.ID, Family: item.Family, Stratum: item.Stratum, Arm: arm, Repeat: 1, Source: *source, CatalogVersion: catalogVersion, CatalogSHA256: actualCatalogSHA, RecipeVersion: preparation.RecipeVersion, Kind: "controlled_source_smoke_not_retrieval_or_whole_task_benchmark", Usage: map[string]any{"provider_calls": 0, "paid_model_calls": 0, "main_model_tokens": nil, "cached_tokens": nil, "embedding_tokens": nil, "dollars": nil, "embedding_execution": "precomputed real pinned local vectors; separately qualified; lookup latency excludes inference"}}
			if vec != nil && (arm == "E0" || arm == "H0" || arm == "H1") {
				row.ModelVersion = vec.ModelVersion
			}
			if unsupportedMechanics(item) {
				row.Status = "NOT_RUN"
				row.ComparisonStatus = "NOT_RUN"
				row.ComparisonReason = "unsupported transition/scenario mechanics"
				row.Error = "NOT_RUN: midflight/scenario mechanics not implemented; no upfront-state substitution"
				if err := enc.Encode(row); err != nil {
					return err
				}
				continue
			}
			if arm == "C0" || arm == "C1" {
				row.Status = "NOT_RUN"
				row.ComparisonStatus = "NOT_RUN"
				row.Error = "No admitted provider/model/account/call-dollar-expiry manifest; C0 native discovery and C1 preregistered direct tools remain distinct"
				if err := enc.Encode(row); err != nil {
					return err
				}
				continue
			}
			c := fixture.Clone(base)
			e := fixture.Overlay(c, item.Fixture.Status)
			if e != nil {
				row.Status = "NOT_RUN"
				row.ComparisonStatus = "NOT_RUN"
				row.Error = e.Error()
				if err := enc.Encode(row); err != nil {
					return err
				}
				continue
			}
			row.ComparisonStatus = "ELIGIBLE"
			if !representable(c, item) {
				row.ComparisonStatus = "NOT_RUN"
				row.ComparisonReason = "fixture_unrepresentable: frozen catalog lacks declared skill/tool-alternative/dependency inventory; actual source smoke result retained separately"
			}
			service, e := c.Service()
			if e != nil {
				return e
			}
			if policy != nil {
				service.TaskAdmitter = policy
			}
			request := fixture.Request(item.Intent)
			request.TopK = *topK
			request.CatalogVersion = catalogVersion
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
					row.ComparisonStatus = "NOT_RUN"
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
				row.CompleteContext = true
				digest := sha256.Sum256(bytes)
				row.ResultSHA256 = hex.EncodeToString(digest[:])
				for _, contract := range result.Selected.Contracts {
					ref := contract.Ref.ID + "@" + contract.Ref.Version
					if contract.Ref.Kind == ports.KindTool {
						row.Tools = append(row.Tools, ref)
					}
					if contract.Ref.Kind == ports.KindCapability {
						row.Capabilities = append(row.Capabilities, ref)
					}
				}
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

func digestString(value any) bool {
	text, ok := value.(string)
	if !ok || len(text) != 64 {
		return false
	}
	decoded, e := hex.DecodeString(text)
	return e == nil && hex.EncodeToString(decoded) == text && strings.TrimSpace(text) == text
}

func representable(c *fixture.Catalog, item input) bool {
	if item.Fixture.SkillID != "" {
		ref := fixture.Ref(ports.KindSkill, item.Fixture.SkillID)
		if item.Fixture.SkillVersion != "" {
			ref.Version = item.Fixture.SkillVersion
		}
		if _, ok := c.Records[ref]; !ok {
			return false
		}
	}
	available := map[string]bool{}
	tools := map[string]bool{}
	for ref := range c.Records {
		available[ref.ID+"@"+ref.Version] = true
		if ref.Kind == ports.KindTool {
			tools[ref.ID+"@"+ref.Version] = true
		}
	}
	for _, dependency := range item.Fixture.Dependencies {
		if !available[dependency] {
			return false
		}
	}
	if len(item.Fixture.ToolAlternatives) > 0 {
		for _, alternative := range item.Fixture.ToolAlternatives {
			complete := true
			for _, tool := range alternative {
				if !tools[tool] {
					complete = false
				}
			}
			if complete {
				return true
			}
		}
		return false
	}
	return true
}

func readBounded(path string, max int64) ([]byte, error) {
	file, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer func() { _ = file.Close() }()
	info, e := file.Stat()
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Size() > max {
		return nil, errors.New("evaluation input must be a bounded regular file")
	}
	raw, e := io.ReadAll(io.LimitReader(file, max+1))
	if e != nil {
		return nil, e
	}
	if int64(len(raw)) > max {
		return nil, errors.New("evaluation input exceeds byte budget")
	}
	return raw, nil
}

func unsupportedMechanics(item input) bool {
	for _, raw := range []json.RawMessage{item.Fixture.Transition, item.Fixture.Scenario} {
		if len(raw) > 0 && strings.TrimSpace(string(raw)) != "null" {
			return true
		}
	}
	return false
}
