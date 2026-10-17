// compositionqual emits deterministic inventories and raw synthetic observations.
// It never supplies expected labels or makes qualification/adoption decisions.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

type configuration struct {
	Schema     string `json:"schema_version"`
	Route      string `json:"route"`
	Samples    int    `json:"warm_samples_per_query_class"`
	Percentile string `json:"percentile_method"`
	Runs       []struct {
		Records    int `json:"logical_records"`
		Partitions int `json:"private_partitions"`
	} `json:"scale_runs"`
	Kinds            map[string]int `json:"kind_percent"`
	CommonPercent    int            `json:"common_term_percent"`
	UpdateRecords    int            `json:"update_records"`
	QueryClasses     []string       `json:"query_classes"`
	AdditionalChecks []string       `json:"additional_checks"`
	Composition      struct {
		Inputs         string          `json:"inputs"`
		Cases          int             `json:"cases"`
		FixedClock     string          `json:"fixed_clock_utc"`
		MaxAge         int             `json:"authority_max_age_seconds"`
		ValidFor       int             `json:"authority_valid_for_seconds"`
		StaleAge       int             `json:"stale_checked_at_seconds_before_now"`
		MaxBytes       int             `json:"default_max_response_bytes"`
		Task           json.RawMessage `json:"task"`
		Adapter        string          `json:"binding_adapter_version"`
		CallerAdapter  string          `json:"caller_adapter"`
		Policy         string          `json:"policy_generation"`
		TaskGeneration string          `json:"task_generation"`
	} `json:"composition"`
	Execution struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
		Go           string `json:"go"`
		CPUs         int    `json:"cpu_limit"`
		Memory       int64  `json:"memory_limit_bytes"`
		Deadline     int    `json:"deadline_seconds_per_run"`
	} `json:"execution"`
}

func loadConfig(path string) (configuration, []byte, error) {
	var c configuration
	b, e := os.ReadFile(path)
	if e != nil {
		return c, nil, e
	}
	e = json.Unmarshal(b, &c)
	if e == nil && (c.Schema != "gist.composition-evaluation/v1" || c.Route != "lexical-scoped-index" || c.Samples != 100 || c.Composition.Cases != 19 || c.Composition.MaxAge != 60 || c.Composition.ValidFor != 60 || c.Composition.StaleAge != 61 || c.Composition.MaxBytes != 1<<20 || c.Composition.Adapter != "1.0.0" || c.Composition.CallerAdapter != "identity-structured/v1" || c.UpdateRecords != 100) {
		e = fmt.Errorf("unsupported evaluation configuration")
	}
	return c, b, e
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	flags := flag.NewFlagSet("compositionqual", flag.ContinueOnError)
	mode := flags.String("mode", "inventory", "inventory, cohort, or scale")
	configPath := flags.String("config", "tools/evaluation/composition-cohort/primary-config.json", "frozen configuration")
	inputsPath := flags.String("inputs", "tools/evaluation/composition-cohort/inputs.jsonl", "scenario inputs")
	out := flags.String("out", "-", "output path; - writes stdout")
	freeze := flags.String("freeze", "", "reviewed SHA256 manifest (required for measured runs)")
	records := flags.Int("records", 0, "logical records for one scale process")
	partitions := flags.Int("partitions", 0, "private partitions for one scale process")
	source := flags.String("source", "", "exact source/archive identity for measured runs")
	if e := flags.Parse(os.Args[1:]); e != nil {
		return e
	}
	c, configBytes, e := loadConfig(*configPath)
	if e != nil {
		return e
	}
	inputs, inputBytes, e := loadInputs(*inputsPath)
	if e != nil {
		return e
	}
	if len(inputs) != c.Composition.Cases {
		return fmt.Errorf("case count differs from config")
	}
	var result any
	switch *mode {
	case "inventory":
		result, e = inventory(c, configBytes, inputBytes, inputs)
	case "cohort", "scale":
		if *freeze == "" || *source == "" {
			return fmt.Errorf("measured runs require reviewed freeze manifest and exact source identity")
		}
		// Execution is implemented separately after the inventory checkpoint.
		_ = records
		_ = partitions
		return fmt.Errorf("measurement runner not yet implemented")
	default:
		return fmt.Errorf("unknown mode %q", *mode)
	}
	if e != nil {
		return e
	}
	b, e := json.MarshalIndent(result, "", "  ")
	if e != nil {
		return e
	}
	b = append(b, '\n')
	if *out == "-" {
		_, e = os.Stdout.Write(b)
		return e
	}
	return os.WriteFile(*out, b, 0644)
}
