package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/zhoushoujianwork/easyeda-agent/internal/connectivity"
)

var errLibLayoutBudget = errors.New("candidate search budget exhausted")

// The electrical graph and measured poses are immutable. Only translations,
// routes and naming markers are searched, on a bounded five-raw grid.
func planLibLayout(input libLayoutSource) (*schCompositionSource, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var src libLayoutSource
	if err = json.Unmarshal(raw, &src); err != nil {
		return nil, err
	}
	fail := func(format string, args ...any) (*schCompositionSource, error) {
		return nil, fmt.Errorf("unresolved: "+format, args...)
	}
	if src.SchemaVersion != 1 || src.Connectivity.ProjectID == "" || src.Connectivity.DocumentID == "" || !plBoxValid(src.Sheet) {
		return fail("schemaVersion:1, target project/page and measured sheet are required")
	}
	if _, _, err := schCompositionUsableBounds(src.Sheet, src.SheetBorder); err != nil {
		return fail("%v", err)
	}
	budget := src.MaxCandidates
	if budget == 0 {
		budget = 20000
	}
	if budget < 1 || budget > 1000000 {
		return fail("maxCandidates must be 1..1000000 (default 20000)")
	}
	for _, k := range src.Keepouts {
		if !plBoxValid(k) || !boxInside(k, src.Sheet) {
			return fail("invalid keepout")
		}
	}
	d := src.Connectivity
	if err = d.Validate(); err != nil {
		return fail("%v", err)
	}
	byID, byRef := map[string]connectivity.Component{}, map[string]connectivity.Component{}
	netNames, names := map[string]string{}, map[string]bool{}
	pinNet := map[string]map[string]string{}
	for _, n := range d.Nets {
		if strings.TrimSpace(n.Name) == "" || names[n.Name] {
			return fail("net %s needs a unique nonempty name", n.ID)
		}
		netNames[n.ID], names[n.Name] = n.Name, true
	}
	for _, c := range d.Components {
		if c.Device.LibraryUUID == "" || !isDeviceLibraryUUID(c.Device.UUID) || len(c.Pins) == 0 {
			return fail("%s needs library identity and complete physical pins", c.Ref)
		}
		byID[c.ID], byRef[c.Ref], pinNet[c.ID] = c, c, map[string]string{}
	}
	for _, edge := range d.Connections {
		pinNet[edge.ComponentID][edge.PinNumber] = edge.NetID
	}
	for _, c := range d.Components {
		for _, q := range c.Pins {
			states := 0
			if pinNet[c.ID][q.Number] != "" {
				states++
			}
			if q.NoConnected {
				states++
			}
			if q.ConnectionState == "unconnected" {
				states++
			}
			if states != 1 {
				return fail("%s.%s must have exactly one net or explicit NC or unconnected intent", c.Ref, q.Number)
			}
		}
	}
	measured := map[string]powerLayoutPlacement{}
	for _, m := range src.Measurements {
		c, ok := byRef[m.Designator]
		if !ok || measured[c.ID].Designator != "" {
			return fail("unknown/duplicate measurement %s", m.Designator)
		}
		if !plBoxValid(m.BBox) || !plGrid(m.X) || !plGrid(m.Y) || !plFinite(m.Rotation) || math.Mod(m.Rotation, 90) != 0 || len(m.Pins) != len(c.Pins) {
			return fail("%s incomplete measured geometry", c.Ref)
		}
		pins := map[string]connectivity.Pin{}
		for _, q := range c.Pins {
			pins[q.Number] = q
		}
		seen := map[string]bool{}
		for _, q := range m.Pins {
			cp, exists := pins[q.Number]
			if !exists || seen[q.Number] || cp.Name != q.Name || !plGrid(q.X) || !plGrid(q.Y) || q.Net != netNames[pinNet[c.ID][q.Number]] {
				return fail("%s.%s measured pin differs from canonical IR", c.Ref, q.Number)
			}
			if _, err := libPinSide(q, m.BBox); err != nil {
				return fail("%s: %v", c.Ref, err)
			}
			seen[q.Number] = true
		}
		measured[c.ID] = m
	}
	if len(measured) != len(d.Components) {
		return fail("measurements must cover every component")
	}
	modules, owners := map[string]connectivity.Module{}, map[string]string{}
	for _, m := range d.Modules {
		if m.ID == "" || modules[m.ID].ID != "" {
			return fail("unknown/duplicate canonical module %s", m.ID)
		}
		for _, id := range append(append([]string{}, m.CoreComponents...), m.PeripheralComponents...) {
			if byID[id].ID == "" || owners[id] != "" {
				return fail("unknown/repeated module member %s", id)
			}
			owners[id] = m.ID
		}
		modules[m.ID] = m
	}
	if len(owners) != len(d.Components) {
		return fail("every component must belong to one module")
	}
	result := &schCompositionSource{SchemaVersion: 1, Connectivity: d, Sheet: src.Sheet, SheetBorder: src.SheetBorder, Keepouts: src.Keepouts}
	seenModules := map[string]bool{}
	for _, intent := range src.LayoutModules {
		cm, ok := modules[intent.ID]
		if !ok || seenModules[intent.ID] {
			return fail("unknown/duplicate layout module %s", intent.ID)
		}
		seenModules[intent.ID] = true
		rootOK := false
		for _, id := range cm.CoreComponents {
			rootOK = rootOK || id == intent.CoreComponentID
		}
		if !rootOK {
			return fail("module %s coreComponentId must name a canonical core", intent.ID)
		}
		members := append(append([]string{}, cm.CoreComponents...), cm.PeripheralComponents...)
		netPolicies := map[string]string{}
		usedNets := map[string]bool{}
		for _, id := range members {
			for _, n := range pinNet[id] {
				usedNets[n] = true
			}
		}
		for n := range usedNets {
			switch intent.NetPolicies[n] {
			case "direct", "local_power", "local_ground", "module_port":
				netPolicies[netNames[n]] = intent.NetPolicies[n]
			default:
				return fail("module %s net %s needs an explicit supported policy", intent.ID, n)
			}
		}
		for n := range intent.NetPolicies {
			if !usedNets[n] {
				return fail("module %s unknown/unused policy %s", intent.ID, n)
			}
		}
		hints := map[string]libLayoutPeripheral{}
		for _, h := range intent.Peripherals {
			if owners[h.ComponentID] != intent.ID || h.ComponentID == intent.CoreComponentID || hints[h.ComponentID].ComponentID != "" {
				return fail("invalid/duplicate peripheral hint %s", h.ComponentID)
			}
			if h.AttachTo != nil && (owners[h.AttachTo.ComponentID] != intent.ID || h.AttachTo.ComponentID == h.ComponentID) {
				return fail("%s attachment target must be another module member", h.ComponentID)
			}
			hints[h.ComponentID] = h
		}
		core := measured[intent.CoreComponentID]
		core = plTranslate(core, -core.X, -core.Y)
		p := powerLayoutPlan{Placements: []powerLayoutPlacement{core}}
		placed := map[string]powerLayoutPlacement{intent.CoreComponentID: core}
		pending := []string{}
		for _, id := range members {
			if id != intent.CoreComponentID {
				pending = append(pending, id)
			}
		}
		for len(pending) > 0 {
			progress := false
			var lastErr error
			for i, id := range pending {
				pairs, e := libAttachmentPairs(id, measured[id], hints[id], placed, members, netPolicies)
				if e != nil {
					return fail("%s: %v", id, e)
				}
				if len(pairs) == 0 {
					continue
				}
				var next *powerLayoutPlan
				for _, pair := range pairs {
					next, lastErr = libPlacePeripheral(p, measured[id], pair, netPolicies, &budget)
					if errors.Is(lastErr, errLibLayoutBudget) {
						return fail("module %s component %s: %v; revise constraints or explicitly increase maxCandidates", intent.ID, id, lastErr)
					}
					if next != nil {
						break
					}
				}
				if next == nil {
					continue
				}
				p = *next
				placed[id] = p.Placements[len(p.Placements)-1]
				pending = append(pending[:i], pending[i+1:]...)
				progress = true
				break
			}
			if !progress {
				return fail("module %s cannot place %v in 400 raw outward / 200 raw lateral search (disconnected/cyclic attachment or collision): %v", intent.ID, pending, lastErr)
			}
		}
		p.Flags = nil
		if err = libJoinDirectNets(&p, netPolicies); err != nil {
			return fail("module %s: %v", intent.ID, err)
		}
		if err = libNameIslands(&p, netPolicies); err != nil {
			return fail("module %s: %v", intent.ID, err)
		}
		if err = validateLibGeometry(&p); err != nil {
			return fail("module %s: %v", intent.ID, err)
		}
		result.Modules = append(result.Modules, schCompositionModule{ID: intent.ID, Title: intent.Title, Placements: p.Placements, Wires: p.Wires, Flags: p.Flags})
	}
	if len(seenModules) != len(modules) {
		return fail("layoutModules must cover every canonical module")
	}
	if _, err = planSchComposition(*result); err != nil {
		return fail("composition validation: %v", err)
	}
	return result, nil
}

type libAttachmentPair struct {
	host, own powerLayoutPin
	side      string
	rank      int
}

func libAttachmentPairs(id string, own powerLayoutPlacement, hint libLayoutPeripheral, placed map[string]powerLayoutPlacement, order []string, policies map[string]string) ([]libAttachmentPair, error) {
	if hint.PinNumber != "" {
		if _, ok := libPin(own, hint.PinNumber); !ok {
			return nil, fmt.Errorf("unknown peripheral pin %s", hint.PinNumber)
		}
	}
	if hint.AttachTo != nil {
		if _, ok := placed[hint.AttachTo.ComponentID]; !ok {
			return nil, nil
		}
	}
	pairs := []libAttachmentPair{}
	for _, hostID := range order {
		host, ok := placed[hostID]
		if !ok {
			continue
		}
		if hint.AttachTo != nil && hint.AttachTo.ComponentID != hostID {
			continue
		}
		foundPin := hint.AttachTo == nil
		for _, hp := range host.Pins {
			if hint.AttachTo != nil && hp.Number != hint.AttachTo.PinNumber {
				continue
			}
			foundPin = true
			if hp.Net == "" {
				continue
			}
			if hint.AttachTo == nil && policies[hp.Net] == "local_ground" {
				continue
			}
			side, err := libPinSide(hp, host.BBox)
			if err != nil {
				return nil, err
			}
			for _, op := range own.Pins {
				if op.Net == "" || hp.Net != op.Net || (hint.PinNumber != "" && hint.PinNumber != op.Number) {
					continue
				}
				rank := 0
				if policies[hp.Net] == "local_power" {
					rank = 1
				}
				pairs = append(pairs, libAttachmentPair{hp, op, side, rank})
			}
		}
		if !foundPin {
			return nil, fmt.Errorf("unknown attachTo pin %s.%s", hostID, hint.AttachTo.PinNumber)
		}
	}
	if hint.AttachTo != nil && len(pairs) == 0 {
		return nil, fmt.Errorf("attachTo has no shared connected pin on %s", id)
	}
	// A reference chooses geometry, not electrical intent. Multiple already-
	// connected candidates are legal; authored member/pin order breaks ties.
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].rank < pairs[j].rank })
	return pairs, nil
}

func libPlacePeripheral(current powerLayoutPlan, measured powerLayoutPlacement, pair libAttachmentPair, policies map[string]string, budget *int) (*powerLayoutPlan, error) {
	current.Flags = nil // Marker reservations are recalculated for each candidate.
	var lastErr error
	// Minimize connection length first; prefer straight candidates at equal cost.
	for cost := 20.0; cost <= 600; cost += 5 {
		for lateral := 0.0; lateral <= math.Min(200, cost-20); lateral += 5 {
			distance := cost - lateral
			if distance > 400 {
				continue
			}
			for _, sign := range []float64{1, -1} {
				if lateral == 0 && sign < 0 {
					continue
				}
				if *budget <= 0 {
					return nil, errLibLayoutBudget
				}
				*budget -= 1
				x, y := endpointFor(pair.host.X, pair.host.Y, distance, pair.side)
				if pair.side == "left" || pair.side == "right" {
					y += sign * lateral
				} else {
					x += sign * lateral
				}
				c := plTranslate(measured, x-pair.own.X, y-pair.own.Y)
				trial := current
				trial.Placements = append(append([]powerLayoutPlacement{}, current.Placements...), c)
				if lastErr = validateLibGeometry(&trial); lastErr != nil {
					continue
				}
				q, _ := libPin(c, pair.own.Number)
				for _, route := range libRoutes(pair.host, q) {
					candidate := trial
					candidate.Wires = libAppendRoute(current.Wires, route)
					if lastErr = validateLibGeometry(&candidate); lastErr != nil {
						continue
					}
					if lastErr = libNameIslands(&candidate, policies); lastErr != nil {
						continue
					}
					return &candidate, nil
				}
			}
		}
	}
	return nil, lastErr
}

// Each island needs exactly one real naming lead. Local rail policies permit
// multiple islands; a direct net is joined before this function's final call.
type libIsland struct {
	net  string
	pins []powerLayoutPin
}

func libIslands(p *powerLayoutPlan) []libIsland {
	parent := make([]int, len(p.Wires))
	for i := range parent {
		parent[i] = i
	}
	var root func(int) int
	root = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for i, w := range p.Wires {
		for j, v := range p.Wires[:i] {
			if w.Net == v.Net && plSegmentsMeet(w.Points[0], w.Points[1], v.Points[0], v.Points[1]) {
				parent[root(i)] = root(j)
			}
		}
	}
	indices := map[string]int{}
	islands := []libIsland{}
	for _, c := range p.Placements {
		for _, q := range c.Pins {
			if q.Net == "" {
				continue
			}
			key := fmt.Sprintf("%q:pin:%g,%g", q.Net, q.X, q.Y)
			for i, w := range p.Wires {
				if w.Net == q.Net && plOnSegment([2]float64{q.X, q.Y}, w.Points[0], w.Points[1]) {
					key = fmt.Sprintf("%q:wire:%d", q.Net, root(i))
					break
				}
			}
			index, ok := indices[key]
			if !ok {
				index = len(islands)
				indices[key] = index
				islands = append(islands, libIsland{net: q.Net})
			}
			islands[index].pins = append(islands[index].pins, q)
		}
	}
	return islands
}

func libNameIslands(p *powerLayoutPlan, policies map[string]string) error {
	p.Flags = nil
	islands := libIslands(p)
	// Ground constraints are tighter than signal naming at dense core pins.
	sort.SliceStable(islands, func(i, j int) bool {
		return policies[islands[i].net] == "local_ground" && policies[islands[j].net] != "local_ground"
	})
	for _, island := range islands {
		kind := "net_port_bi"
		if policies[island.net] == "local_ground" {
			kind = "ground"
		}
		if policies[island.net] == "local_power" {
			kind = "power"
		}
		found := false
		for _, q := range island.pins {
			if libPlaceMarker(p, q, kind) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("net %s has no safe naming lead for island at pin %s (%g,%g)", island.net, island.pins[0].Number, island.pins[0].X, island.pins[0].Y)
		}
	}
	return nil
}

func libJoinDirectNets(p *powerLayoutPlan, policies map[string]string) error {
	for {
		islands := libIslands(p)
		type edge struct {
			a, b   powerLayoutPin
			length float64
		}
		edges := []edge{}
		for i, a := range islands {
			if policies[a.net] != "direct" && policies[a.net] != "module_port" {
				continue
			}
			for _, b := range islands[:i] {
				if a.net == b.net {
					for _, x := range a.pins {
						for _, y := range b.pins {
							edges = append(edges, edge{x, y, math.Abs(x.X-y.X) + math.Abs(x.Y-y.Y)})
						}
					}
				}
			}
		}
		if len(edges) == 0 {
			return nil
		}
		sort.SliceStable(edges, func(i, j int) bool { return edges[i].length < edges[j].length })
		joined := false
		for _, e := range edges {
			for _, route := range libRoutes(e.a, e.b) {
				trial := *p
				trial.Wires = libAppendRoute(p.Wires, route)
				if validateLibGeometry(&trial) == nil && len(libIslands(&trial)) < len(islands) {
					*p = trial
					joined = true
					break
				}
			}
			if joined {
				break
			}
		}
		if !joined {
			return fmt.Errorf("cannot route direct net %s between measured pins without crossing obstacles; revise attachment or measured pose", edges[0].a.Net)
		}
	}
}
