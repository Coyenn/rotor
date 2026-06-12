package assets

// Action is what sync will do with one scanned file.
type Action int

const (
	ActionCreate    Action = iota // not in the lockfile: upload as a new asset
	ActionUpdate                  // hash changed: replace content, keep the asset id
	ActionUnchanged               // hash matches the lockfile: no-op
)

func (a Action) String() string {
	switch a {
	case ActionCreate:
		return "create"
	case ActionUpdate:
		return "update"
	case ActionUnchanged:
		return "unchanged"
	}
	return "unknown"
}

// PlanItem pairs a scanned file with its action. AssetID carries the existing
// id for updates and unchanged files (zero for creates).
type PlanItem struct {
	File    File
	Action  Action
	AssetID int64
}

// Plan is the full sync plan, in stable path order.
type Plan struct {
	Items   []PlanItem
	Skipped []string // unknown-extension files reported by Scan
}

// BuildPlan diffs the scan result against the lockfile. New files are
// creates, changed hashes are updates (content replacement keeps the id
// stable), matching hashes are unchanged.
func BuildPlan(scan *ScanResult, lock *Lockfile) *Plan {
	p := &Plan{Skipped: scan.Skipped}
	for _, f := range scan.Files {
		entry, ok := lock.Assets[f.Path]
		switch {
		case !ok:
			p.Items = append(p.Items, PlanItem{File: f, Action: ActionCreate})
		case entry.Hash != f.Hash:
			p.Items = append(p.Items, PlanItem{File: f, Action: ActionUpdate, AssetID: entry.AssetID})
		default:
			p.Items = append(p.Items, PlanItem{File: f, Action: ActionUnchanged, AssetID: entry.AssetID})
		}
	}
	return p
}

// Count returns how many plan items carry the given action.
func (p *Plan) Count(a Action) int {
	n := 0
	for _, it := range p.Items {
		if it.Action == a {
			n++
		}
	}
	return n
}

// Changes returns the number of items that need an upload (creates + updates).
func (p *Plan) Changes() int {
	return p.Count(ActionCreate) + p.Count(ActionUpdate)
}
