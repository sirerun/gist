package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	c "github.com/sirerun/gist/catalog/composition"
)

type freezeManifest struct {
	Schema string            `json:"schema_version"`
	Files  map[string]string `json:"files"`
}

func verifyFreeze(path, configPath, inputsPath string) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	var m freezeManifest
	if e = json.Unmarshal(b, &m); e != nil {
		return e
	}
	if m.Schema != "gist.composition-freeze/v1" || len(m.Files) == 0 {
		return errors.New("invalid freeze manifest")
	}
	for _, required := range []string{configPath, inputsPath} {
		if _, ok := m.Files[filepath.ToSlash(filepath.Clean(required))]; !ok {
			return fmt.Errorf("freeze omits %s", required)
		}
	}
	for name, expected := range m.Files {
		if filepath.IsAbs(name) || strings.HasPrefix(filepath.Clean(name), "..") {
			return errors.New("freeze path escapes repository")
		}
		b, e := os.ReadFile(name)
		if e != nil {
			return e
		}
		if digest(b).Value != expected {
			return fmt.Errorf("freeze digest changed: %s", name)
		}
	}
	return nil
}

type hostEvidence struct {
	OS              string `json:"os"`
	Architecture    string `json:"architecture"`
	Go              string `json:"go"`
	Kernel          string `json:"kernel"`
	CPUs            int    `json:"visible_cpus"`
	GOMAXPROCS      int    `json:"gomaxprocs"`
	CPUMax          string `json:"cgroup_cpu_max"`
	MemoryMax       string `json:"cgroup_memory_max"`
	DeadlineSeconds int    `json:"deadline_seconds"`
}

func host(c configuration) hostEvidence {
	read := func(path string) string {
		b, e := os.ReadFile(path)
		if e != nil {
			return "unavailable: " + e.Error()
		}
		return strings.TrimSpace(string(b))
	}
	base := "/sys/fs/cgroup"
	if b, e := os.ReadFile("/proc/self/cgroup"); e == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "0::") {
				base = filepath.Join(base, strings.TrimPrefix(line, "0::"))
				break
			}
		}
	}
	return hostEvidence{runtime.GOOS, runtime.GOARCH, runtime.Version(), read("/proc/sys/kernel/osrelease"), runtime.NumCPU(), runtime.GOMAXPROCS(0), read(filepath.Join(base, "cpu.max")), read(filepath.Join(base, "memory.max")), c.Execution.Deadline}
}
func validateHost(c configuration) error {
	if runtime.GOOS != c.Execution.OS || runtime.GOARCH != c.Execution.Architecture || runtime.Version() != c.Execution.Go {
		return fmt.Errorf("target mismatch: %s/%s %s; frozen target %s/%s %s", runtime.GOOS, runtime.GOARCH, runtime.Version(), c.Execution.OS, c.Execution.Architecture, c.Execution.Go)
	}
	if runtime.GOMAXPROCS(0) > c.Execution.CPUs {
		return errors.New("GOMAXPROCS exceeds frozen CPU ceiling")
	}
	return nil
}

type cohortObservation struct {
	ID            string       `json:"id"`
	Scenario      string       `json:"scenario"`
	Source        string       `json:"source"`
	Result        c.Result     `json:"result"`
	Error         string       `json:"error"`
	ElapsedNS     int64        `json:"elapsed_ns"`
	ResponseBytes int          `json:"response_bytes"`
	Counts        counters     `json:"counts"`
	SetupNS       int64        `json:"setup_ns"`
	SetupCounts   counters     `json:"setup_counts"`
	Host          hostEvidence `json:"host"`
}

func runCohort(ctx context.Context, cfg configuration, inputs []scenario, source string, emit func(any) error) error {
	for _, input := range inputs {
		started := time.Now()
		f, e := makeFixture(cfg, input)
		if e != nil {
			return e
		}
		service, e := f.service(ctx)
		ob := cohortObservation{ID: input.ID, Scenario: input.Scenario, Source: source, SetupNS: time.Since(started).Nanoseconds(), SetupCounts: *f.store.counts, Host: host(cfg)}
		if e != nil {
			ob.Error = "setup: " + e.Error()
			if e = emit(ob); e != nil {
				return e
			}
			continue
		}
		*f.store.counts = counters{}
		requestCtx := ctx
		var cancel context.CancelFunc
		if input.Scenario == "cancelled_request" {
			requestCtx, cancel = context.WithCancel(ctx)
			cancel()
		}
		started = time.Now()
		ob.Result, e = service.Prepare(requestCtx, f.request)
		ob.ElapsedNS = time.Since(started).Nanoseconds()
		ob.Counts = *f.store.counts
		if e != nil {
			ob.Error = e.Error()
		}
		ob.ResponseBytes = len(encode(ob.Result))
		if e = emit(ob); e != nil {
			return e
		}
	}
	return nil
}
