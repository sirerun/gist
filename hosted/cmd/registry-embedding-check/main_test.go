package main

import (
	"math"
	"testing"
)

func TestNumericalQualificationRejectsInvalidOutputs(t *testing.T) {
	unit := make([]float64, 768)
	unit[0] = 1
	cases := map[string][][]float64{
		"missing":   nil,
		"extra":     {unit, unit},
		"truncated": {unit[:767]},
		"zero":      {make([]float64, 768)},
	}
	for _, bad := range []float64{math.NaN(), math.Inf(1), 2} {
		vector := append([]float64(nil), unit...)
		vector[0] = bad
		if _, err := validatePair([][]float64{vector}, [][]float64{unit}); err == nil {
			t.Fatal("invalid vector accepted")
		}
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := validatePair(bad, [][]float64{unit}); err == nil {
				t.Fatal("invalid query accepted")
			}
			if _, err := validatePair([][]float64{unit}, bad); err == nil {
				t.Fatal("invalid document accepted")
			}
		})
	}
	// Orthogonal and opposite valid vectors qualify: this command intentionally
	// makes no retrieval relevance assertion about its synthetic input pair.
	orthogonal := make([]float64, 768)
	orthogonal[1] = 1
	cosine, err := validatePair([][]float64{unit}, [][]float64{orthogonal})
	if err != nil || cosine != 0 {
		t.Fatalf("valid orthogonal output rejected: %v", err)
	}
	opposite := make([]float64, 768)
	opposite[0] = -1
	cosine, err = validatePair([][]float64{unit}, [][]float64{opposite})
	if err != nil || cosine != -1 {
		t.Fatalf("valid opposite output rejected: %v", err)
	}
}
