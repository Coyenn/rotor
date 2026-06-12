// Package pack turns a Rojo project into a distributable artifact: a real Roblox
// model (.rbxm/.rbxmx, via `rojo build`) or a single self-reconstructing Luau script
// that rebuilds the instance tree + a require polyfill at runtime (wax/azalea style).
// The authoritative instance tree comes from `rojo build` (so all of Rojo's
// file->instance semantics — init merging, .meta.json, sub-extensions, .txt ->
// StringValue — are handled for us).
package pack

import "encoding/xml"

// Instance is a node of the reconstructed Roblox instance tree.
type Instance struct {
	ClassName string
	Name      string
	Source    string // script classes (ModuleScript/Script/LocalScript)
	Value     string // StringValue
	Children  []*Instance

	id int // assigned during Luau emission
}

// IsScript reports whether the instance is one of the runnable/requirable script
// classes.
func (i *Instance) IsScript() bool {
	switch i.ClassName {
	case "ModuleScript", "Script", "LocalScript":
		return true
	}
	return false
}

// --- .rbxmx (XML model) parsing ---

type xmlRoblox struct {
	XMLName xml.Name  `xml:"roblox"`
	Items   []xmlItem `xml:"Item"`
}

type xmlItem struct {
	Class string    `xml:"class,attr"`
	Props xmlProps  `xml:"Properties"`
	Items []xmlItem `xml:"Item"`
}

type xmlProps struct {
	Strings   []xmlProp `xml:"string"`
	Protected []xmlProp `xml:"ProtectedString"`
}

type xmlProp struct {
	Name string `xml:"name,attr"`
	Text string `xml:",chardata"`
}

// ParseRbxmx parses a Roblox XML model (as produced by `rojo build -o x.rbxmx`) into
// the top-level instance trees.
func ParseRbxmx(data []byte) ([]*Instance, error) {
	var root xmlRoblox
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	out := make([]*Instance, 0, len(root.Items))
	for i := range root.Items {
		out = append(out, convertItem(&root.Items[i]))
	}
	return out, nil
}

func convertItem(it *xmlItem) *Instance {
	inst := &Instance{ClassName: it.Class}
	for _, p := range it.Props.Strings {
		applyProp(inst, p)
	}
	for _, p := range it.Props.Protected {
		applyProp(inst, p)
	}
	for i := range it.Items {
		inst.Children = append(inst.Children, convertItem(&it.Items[i]))
	}
	return inst
}

func applyProp(inst *Instance, p xmlProp) {
	switch p.Name {
	case "Name":
		inst.Name = p.Text
	case "Source":
		inst.Source = p.Text
	case "Value":
		inst.Value = p.Text
	}
}
