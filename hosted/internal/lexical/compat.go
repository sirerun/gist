// Package lexical preserves the hosted API using the shared catalog implementation.
package lexical

import shared "github.com/sirerun/gist/internal/cataloglexical"

type Document = shared.Document
type Match = shared.Match
type Index = shared.Index

const WeightName = shared.WeightName
const WeightTaxonomy = shared.WeightTaxonomy
const WeightDescription = shared.WeightDescription

var Terms = shared.Terms
var Query = shared.Query
var Stem = shared.Stem
var DocumentFromMetadata = shared.DocumentFromMetadata
var NewIndex = shared.NewIndex
var Score = shared.Score
