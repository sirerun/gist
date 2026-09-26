package publication

import (
	"errors"
	"fmt"
	"time"

	"github.com/sirerun/gist/hosted/internal/packages"
)

type Decision string

const (
	Approved Decision = "approved"
	Rejected Decision = "rejected"
	Held     Decision = "held"
)

type Review struct {
	ReviewerID string
	Decision   Decision
	Evidence   map[string]string
	At         time.Time
}

func ReviewPackage(p packages.Package, reviewer string, evidence map[string]string) (Review, error) {
	if reviewer == "" {
		return Review{}, errors.New("publication: reviewer is required")
	}
	if err := packages.ScreenText(p); err != nil {
		return Review{ReviewerID: reviewer, Decision: Held, Evidence: evidence, At: time.Now().UTC()}, err
	}
	if len(evidence) == 0 {
		return Review{}, errors.New("publication: review evidence is required")
	}
	return Review{ReviewerID: reviewer, Decision: Approved, Evidence: evidence, At: time.Now().UTC()}, nil
}
func (r Review) Validate() error {
	if r.ReviewerID == "" || r.At.IsZero() {
		return errors.New("publication: incomplete review")
	}
	if r.Decision != Approved && r.Decision != Rejected && r.Decision != Held {
		return fmt.Errorf("publication: invalid review decision %q", r.Decision)
	}
	return nil
}
