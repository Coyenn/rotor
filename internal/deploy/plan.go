package deploy

import (
	"sort"
	"strings"
)

// Op is what the plan decided to do with one resource.
type Op string

const (
	OpCreate Op = "create"
	OpUpdate Op = "update"
	OpNoop   Op = "no-op"
	OpDelete Op = "delete"
	// OpBlockedDelete marks a resource that is in state but gone from the
	// config while --allow-deletes was NOT given: plan shows it as blocked,
	// apply refuses to run.
	OpBlockedDelete Op = "blocked-delete"
)

// Step is one planned action. Resource is nil for (blocked) deletes — the
// resource no longer exists in the config; only its state entry remains.
type Step struct {
	Op         Op
	Ref        ResourceRef
	Resource   *Resource
	InputsHash string // desired-inputs hash; empty for deletes
}

// PlanOptions tunes plan construction.
type PlanOptions struct {
	// AllowDeletes lets resources that vanished from the config be planned
	// as real deletes; without it they appear as OpBlockedDelete.
	AllowDeletes bool
}

// Plan is an ordered set of steps plus tallies for the summary line.
type Plan struct {
	Steps          []Step
	Creates        int
	Updates        int
	Deletes        int
	BlockedDeletes int
	Noops          int
}

// HasChanges reports whether applying the plan would do anything (blocked
// deletes don't count: apply refuses to run while they exist).
func (p *Plan) HasChanges() bool { return p.Creates+p.Updates+p.Deletes > 0 }

// BuildPlan diffs the desired resource graph against the recorded state and
// returns the ordered steps: desired resources in dependency order
// (create when absent from state, update when the canonical inputs hash
// changed, no-op otherwise), then deletes for state entries no longer in the
// config. BuildPlan is pure — no network, no filesystem.
func BuildPlan(resources []Resource, st *State, opts PlanOptions) (*Plan, error) {
	ordered, err := topoSort(resources)
	if err != nil {
		return nil, err
	}
	if st == nil {
		st = NewState()
	}

	plan := &Plan{}
	desired := make(map[string]bool, len(ordered))
	for i := range ordered {
		r := &ordered[i]
		key := r.Ref().Key()
		desired[key] = true
		hash, err := HashInputs(r.Inputs)
		if err != nil {
			return nil, err
		}
		op := OpCreate
		if prior, ok := st.Resources[key]; ok {
			if prior.InputsHash == hash {
				op = OpNoop
			} else {
				op = OpUpdate
			}
		}
		switch op {
		case OpCreate:
			plan.Creates++
		case OpUpdate:
			plan.Updates++
		case OpNoop:
			plan.Noops++
		}
		plan.Steps = append(plan.Steps, Step{Op: op, Ref: r.Ref(), Resource: r, InputsHash: hash})
	}

	// State entries that no longer exist in the config become deletes,
	// appended after all live steps, in stable key order.
	var gone []string
	for key := range st.Resources {
		if !desired[key] {
			gone = append(gone, key)
		}
	}
	sort.Strings(gone)
	for _, key := range gone {
		ref := parseStateKey(key)
		op := OpDelete
		if opts.AllowDeletes {
			plan.Deletes++
		} else {
			op = OpBlockedDelete
			plan.BlockedDeletes++
		}
		plan.Steps = append(plan.Steps, Step{Op: op, Ref: ref})
	}
	return plan, nil
}

// parseStateKey splits a "<kind>/<name>" state key back into a ref. Names
// may contain slashes (asset paths); kinds never do.
func parseStateKey(key string) ResourceRef {
	if i := strings.Index(key, "/"); i >= 0 {
		return ResourceRef{Kind: key[:i], Name: key[i+1:]}
	}
	return ResourceRef{Kind: key}
}
