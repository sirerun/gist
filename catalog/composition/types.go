// Package composition validates and prepares bounded portable recipes. It never
// executes tools or grants authority; its ports are supplied by the caller.
package composition

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sirerun/gist/catalog/packages"
	"github.com/sirerun/gist/catalog/ports"
)

const Version = "gist.composition/v1"
const AssetPath = "assets/composition.json"
const MaxJSONBytes = 256 << 10

type Limits struct {
	MaxSteps       int `json:"max_steps"`
	MaxIterations  int `json:"max_iterations"`
	MaxCalls       int `json:"max_calls"`
	MaxInputBytes  int `json:"max_input_bytes"`
	MaxOutputBytes int `json:"max_output_bytes"`
	MaxGuideBytes  int `json:"max_guide_bytes"`
}
type Requirement struct {
	Key             string `json:"key"`
	CapabilityID    string `json:"capability_id"`
	ContractVersion string `json:"contract_version"`
	ContractDigest  string `json:"contract_digest"`
	Optional        bool   `json:"optional"`
}
type Group struct {
	ID           string   `json:"id"`
	Requirements []string `json:"requirements"`
}
type Source struct {
	Step    string `json:"step"`
	Pointer string `json:"pointer"`
}
type Iteration struct {
	Step    string `json:"step"`
	Pointer string `json:"pointer"`
	Max     int    `json:"max"`
}
type Step struct {
	ID          string            `json:"id"`
	Requirement string            `json:"requirement"`
	After       []string          `json:"after"`
	Inputs      map[string]Source `json:"inputs"`
	Foreach     *Iteration        `json:"foreach,omitempty"`
}
type Document struct {
	SchemaVersion    string        `json:"schema_version"`
	SkillID          string        `json:"skill_id"`
	SkillVersion     string        `json:"skill_version"`
	Requirements     []Requirement `json:"requirements"`
	ConnectionGroups []Group       `json:"connection_groups"`
	TaskSchema       Schema        `json:"task_schema"`
	Steps            []Step        `json:"steps"`
	Limits           Limits        `json:"limits"`
}

// Schema deliberately supports a closed, bounded subset, not general JSON Schema.
type Schema struct {
	Type                 string            `json:"type"`
	Properties           map[string]Schema `json:"properties,omitempty"`
	Required             []string          `json:"required,omitempty"`
	AdditionalProperties *bool             `json:"additionalProperties,omitempty"`
	Items                *Schema           `json:"items,omitempty"`
	MaxItems             int               `json:"maxItems,omitempty"`
	MaxLength            int               `json:"maxLength,omitempty"`
}
type Contract struct {
	ID, Version   string
	Input, Output Schema
	Bytes         json.RawMessage
}
type Connection struct{ Scope, ID, Version, AccountID, Generation string }

// Candidate is trusted caller-index evidence, never a catalog search result.
type Candidate struct {
	Binding, Tool, Provider                       ports.ArtifactPin
	ContractDigest, AdapterVersion, FixtureDigest string
	Connection                                    Connection
	Admitted                                      bool
}
type Query struct {
	Principal   ports.Principal
	RuntimeID   string
	Requirement Requirement
	Limit       int
}
type Candidates struct {
	Items    []Candidate
	Complete bool
}
type BindingIndex interface {
	Lookup(context.Context, Query) (Candidates, error)
}
type Retriever interface {
	RetrieveSkill(context.Context, ports.Principal, ports.ArtifactPin, packages.Limits) (packages.Package, error)
	Retrieve(context.Context, ports.Principal, ports.ArtifactRef, int64) (packages.Envelope, error)
}
type AuthorityRequest struct {
	Principal              ports.Principal
	RuntimeID, ScopeDigest string
	Task                   json.RawMessage
	Skill                  ports.ArtifactPin
	CompositionDigest      string
	Selected               map[string]Candidate
}
type Decision struct {
	ScopeDigest, PolicyGeneration, TaskGeneration string
	Ready                                         bool
	CheckedAt, ValidUntil                         time.Time
}
type Authority interface {
	Check(context.Context, AuthorityRequest) (Decision, error)
}
type Clock interface{ Now() time.Time }
type Service struct {
	Retriever Retriever
	Index     BindingIndex
	Authority Authority
	Clock     Clock
	MaxAge    time.Duration
}
type Request struct {
	Principal            ports.Principal
	RuntimeID            string
	Skill                ports.ArtifactPin
	Task                 json.RawMessage
	SupportedVersions    []string
	Selections           map[string]ports.ArtifactPin
	ConnectionSelections map[string]Connection
	Limits               Limits
	MaxBytes             int
}
type Finding struct {
	Requirement, Status string
	Selected            *Candidate `json:",omitempty"`
}
type GroupEvidence struct {
	Group      string
	Connection Connection
}
type Tool struct {
	Pin                       ports.ArtifactPin
	Connection                Connection
	InputSchema, OutputSchema json.RawMessage
}
type Bundle struct {
	Document          Document
	CompositionDigest string
	Skill             ports.ArtifactPin
	Contracts         map[string]Contract
	Selected          map[string]Candidate
	Tools             []Tool
	DisabledSteps     []string
	Guide             string
}
type Result struct {
	Status                                        string
	Findings                                      []Finding
	Groups                                        []GroupEvidence
	Bundle                                        *Bundle `json:",omitempty"`
	ScopeDigest, PolicyGeneration, TaskGeneration string
	ValidUntil                                    time.Time
}
