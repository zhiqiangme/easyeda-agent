package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"

	"github.com/spf13/cobra"
	"github.com/zhoushoujianwork/easyeda-agent/internal/connectivity"
)

// buildSchPlan emits the same version-1 playbook consumed by sch apply.
// Only explicit marker additions are supported; never invent wiring intent.
func buildSchPlan(a, b connectivity.Document) (*playbook, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	if err := b.Validate(); err != nil {
		return nil, err
	}
	if a.ProjectID == "" || a.DocumentID == "" || a.ProjectID != b.ProjectID || a.DocumentID != b.DocumentID {
		return nil, fmt.Errorf("matching projectId/documentId required; refresh page snapshots")
	}
	ac, bc := map[string]connectivity.Component{}, map[string]connectivity.Component{}
	for _, c := range a.Components {
		ac[c.ID] = c
	}
	for _, c := range b.Components {
		bc[c.ID] = c
	}
	// An explicit open pin becomes connected when its new marker edge is
	// added. Normalize only that declaration for comparison; all identity,
	// pin inventory, geometry and NC changes remain unsupported.
	oldEdges := map[[2]string]bool{}
	newEdges := map[[2]string]bool{}
	for _, edge := range a.Connections {
		oldEdges[[2]string{edge.ComponentID, edge.PinNumber}] = true
	}
	for _, edge := range b.Connections {
		newEdges[[2]string{edge.ComponentID, edge.PinNumber}] = true
	}
	for id, c := range ac {
		c.Pins = append([]connectivity.Pin(nil), c.Pins...)
		for i, pin := range c.Pins {
			key := [2]string{id, pin.Number}
			if pin.ConnectionState == "unconnected" && !oldEdges[key] && newEdges[key] {
				c.Pins[i].ConnectionState = ""
			}
		}
		ac[id] = c
	}
	if !reflect.DeepEqual(ac, bc) || !reflect.DeepEqual(a.Modules, b.Modules) {
		return nil, fmt.Errorf("component/module changes unsupported; no plan generated")
	}
	an, bn := map[string]connectivity.Net{}, map[string]connectivity.Net{}
	for _, n := range a.Nets {
		an[n.ID] = n
	}
	for _, n := range b.Nets {
		bn[n.ID] = n
	}
	for id, n := range an {
		if bn[id] != n {
			return nil, fmt.Errorf("net removal/rename/change unsupported: %s", id)
		}
	}
	key := func(c connectivity.Connection) [2]string { return [2]string{c.ComponentID, c.PinNumber} }
	old, next := map[[2]string]connectivity.Connection{}, map[[2]string]connectivity.Connection{}
	for _, c := range a.Connections {
		old[key(c)] = c
	}
	for _, c := range b.Connections {
		next[key(c)] = c
	}
	for k, c := range old {
		n, ok := next[k]
		if !ok || n.NetID != c.NetID {
			return nil, fmt.Errorf("disconnect/rewire unsupported: %v", k)
		}
	}
	additions := []connectivity.Connection{}
	for k, c := range next {
		if _, ok := old[k]; !ok {
			switch c.Kind {
			case "power", "ground", "net_port_in", "net_port_out", "net_port_bi":
			default:
				return nil, fmt.Errorf("%v: explicit power/ground/net_port kind required; wire routing is not implemented", k)
			}
			if bn[c.NetID].Name == "" {
				return nil, fmt.Errorf("net name required")
			}
			additions = append(additions, c)
		}
	}
	sort.Slice(additions, func(i, j int) bool {
		a, b := additions[i], additions[j]
		if a.ComponentID != b.ComponentID {
			return a.ComponentID < b.ComponentID
		}
		return a.PinNumber < b.PinNumber
	})
	zero := 0
	stop := false
	p := &playbook{Version: 1, Meta: playbookMeta{Name: "Connectivity additions", Project: a.ProjectID, Doc: a.DocumentID}, Defaults: stepPolicy{Retry: &zero, ContinueOnError: &stop}, Steps: []playbookStep{}}
	state := a
	check := func() {
		copyState := state
		copyState.Connections = append([]connectivity.Connection(nil), state.Connections...)
		p.Steps = append(p.Steps, playbookStep{ID: fmt.Sprintf("check-%03d", len(p.Steps)+1), Action: "schematic.read", Payload: map[string]any{"includeCheck": false}, ExpectedConnectivity: &copyState})
	}
	check()
	state.Nets = b.Nets
	for _, c := range additions {
		p.Steps = append(p.Steps, playbookStep{ID: fmt.Sprintf("connect-%03d", len(p.Steps)+1), Run: "sch autoconnect", Flags: map[string]any{"pin": bc[c.ComponentID].Ref + ":" + c.PinNumber, "net": bn[c.NetID].Name, "kind": c.Kind, "strict": true}})
		state.Connections = append(append([]connectivity.Connection(nil), state.Connections...), c)
		// Copy before updating so already emitted checkpoints and caller input
		// retain their original open-pin declarations.
		state.Components = append([]connectivity.Component(nil), state.Components...)
		for i, part := range state.Components {
			if part.ID != c.ComponentID {
				continue
			}
			state.Components[i].Pins = append([]connectivity.Pin(nil), part.Pins...)
			for j, pin := range part.Pins {
				if pin.Number == c.PinNumber && pin.ConnectionState == "unconnected" {
					state.Components[i].Pins[j].ConnectionState = ""
				}
			}
		}
		check()
	}
	if len(additions) > 0 {
		p.Steps = append(p.Steps, playbookStep{ID: "save", Action: "schematic.save"})
		check()
	}
	return p, nil
}
func newSchPlanCmd(stdout io.Writer) *cobra.Command {
	return &cobra.Command{Use: "plan <before.json> <after.json>", Short: "Generate a guarded sch apply playbook for explicit marker additions (offline)", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		var docs [2]connectivity.Document
		for i, path := range args {
			raw, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			if e = json.Unmarshal(raw, &docs[i]); e != nil {
				return e
			}
		}
		p, e := buildSchPlan(docs[0], docs[1])
		if e != nil {
			return e
		}
		return json.NewEncoder(stdout).Encode(p)
	}}
}
