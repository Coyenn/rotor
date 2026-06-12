package deploy

import (
	"context"
	"fmt"
)

// StepStatus is the outcome of executing one plan step.
type StepStatus string

const (
	StatusApplied   StepStatus = "applied"
	StatusUnchanged StepStatus = "unchanged"
	StatusFailed    StepStatus = "failed"
	// StatusSkipped marks a step whose dependency failed (or was itself
	// skipped); the resource was not touched.
	StatusSkipped StepStatus = "skipped"
)

// StepResult pairs a plan step with what happened when it ran.
type StepResult struct {
	Step   Step
	Status StepStatus
	Err    error
}

// ApplyOptions configures an Apply run.
type ApplyOptions struct {
	Providers  map[string]Provider
	Cloud      CloudClient
	UniverseID int64
	ProjectDir string

	// State is mutated in place as resources apply; SaveState (when set) is
	// called after EVERY state mutation so an interrupted run resumes.
	State     *State
	SaveState func(*State) error

	// OnStep, when set, observes each step as it completes (CLI progress).
	OnStep func(StepResult)
}

// ApplyResult tallies an Apply run. Failed > 0 means at least one resource
// errored; its dependents are counted in Skipped.
type ApplyResult struct {
	Results []StepResult

	Created   int
	Updated   int
	Deleted   int
	Unchanged int
	Failed    int
	Skipped   int
}

// Apply executes a plan in order. Per-resource failures do not abort the
// run: dependents of a failed resource are skipped, independent siblings
// still apply, and every success is persisted to state immediately. Apply
// itself returns an error only for structural problems (blocked deletes in
// the plan, missing provider, state-save failure); resource errors live in
// the result.
func Apply(ctx context.Context, plan *Plan, opts ApplyOptions) (*ApplyResult, error) {
	if plan.BlockedDeletes > 0 {
		return nil, fmt.Errorf("deploy: plan contains %d delete(s); re-run with --allow-deletes to remove resources", plan.BlockedDeletes)
	}
	st := opts.State
	if st == nil {
		st = NewState()
	}
	save := func() error {
		if opts.SaveState == nil {
			return nil
		}
		return opts.SaveState(st)
	}

	dctx := &Ctx{
		Cloud:      opts.Cloud,
		UniverseID: opts.UniverseID,
		ProjectDir: opts.ProjectDir,
		Output: func(ref ResourceRef, key string) (any, bool) {
			entry, ok := st.Resources[ref.Key()]
			if !ok || entry.Outputs == nil {
				return nil, false
			}
			v, ok := entry.Outputs[key]
			return v, ok
		},
	}

	res := &ApplyResult{}
	failed := map[string]bool{} // failed or skipped resource keys
	record := func(step Step, status StepStatus, err error) {
		r := StepResult{Step: step, Status: status, Err: err}
		res.Results = append(res.Results, r)
		if opts.OnStep != nil {
			opts.OnStep(r)
		}
	}

	for _, step := range plan.Steps {
		key := step.Ref.Key()
		switch step.Op {
		case OpNoop:
			res.Unchanged++
			record(step, StatusUnchanged, nil)

		case OpCreate, OpUpdate:
			if dep, bad := failedDep(step.Resource, failed); bad {
				failed[key] = true
				res.Skipped++
				record(step, StatusSkipped, fmt.Errorf("dependency %s failed", dep.Key()))
				continue
			}
			provider, ok := opts.Providers[step.Ref.Kind]
			if !ok {
				return res, fmt.Errorf("deploy: no provider registered for resource kind %q", step.Ref.Kind)
			}
			prior := st.Resources[key]
			var outputs map[string]any
			var err error
			if step.Op == OpCreate {
				outputs, err = provider.Create(ctx, dctx, step.Resource.Inputs, prior)
			} else {
				outputs, err = provider.Update(ctx, dctx, step.Resource.Inputs, prior)
			}
			if err != nil {
				failed[key] = true
				res.Failed++
				record(step, StatusFailed, err)
				continue
			}
			// Updates keep prior outputs the provider did not re-produce
			// (e.g. a config-only update preserving a created id).
			if prior != nil && prior.Outputs != nil {
				merged := make(map[string]any, len(prior.Outputs)+len(outputs))
				for k, v := range prior.Outputs {
					merged[k] = v
				}
				for k, v := range outputs {
					merged[k] = v
				}
				outputs = merged
			}
			st.Resources[key] = &StateEntry{InputsHash: step.InputsHash, Outputs: outputs}
			if err := save(); err != nil {
				return res, err
			}
			if step.Op == OpCreate {
				res.Created++
			} else {
				res.Updated++
			}
			record(step, StatusApplied, nil)

		case OpDelete:
			// Unknown kinds in state (written by a different rotor version)
			// are forgotten without a provider call.
			if provider, ok := opts.Providers[step.Ref.Kind]; ok {
				if err := provider.Delete(ctx, dctx, st.Resources[key]); err != nil {
					failed[key] = true
					res.Failed++
					record(step, StatusFailed, err)
					continue
				}
			}
			delete(st.Resources, key)
			if err := save(); err != nil {
				return res, err
			}
			res.Deleted++
			record(step, StatusApplied, nil)

		case OpBlockedDelete:
			// Unreachable: guarded above. Kept for exhaustiveness.
			return res, fmt.Errorf("deploy: blocked delete reached apply for %s", key)
		}
	}
	return res, nil
}

// failedDep reports whether any direct dependency of r already failed or was
// skipped. Transitive failure propagates naturally: a skipped dependency is
// itself in the failed set.
func failedDep(r *Resource, failed map[string]bool) (ResourceRef, bool) {
	if r == nil {
		return ResourceRef{}, false
	}
	for _, dep := range r.DependsOn {
		if failed[dep.Key()] {
			return dep, true
		}
	}
	return ResourceRef{}, false
}
