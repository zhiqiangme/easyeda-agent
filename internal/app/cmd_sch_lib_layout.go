package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zhoushoujianwork/easyeda-agent/internal/connectivity"
)

type libLayoutAttach struct {
	ComponentID string `json:"componentId"`
	PinNumber   string `json:"pinNumber"`
}
type libLayoutPeripheral struct {
	ComponentID string           `json:"componentId"`
	PinNumber   string           `json:"pinNumber,omitempty"`
	AttachTo    *libLayoutAttach `json:"attachTo,omitempty"`
}
type libLayoutModule struct {
	ID              string                `json:"id"`
	Title           string                `json:"title"`
	CoreComponentID string                `json:"coreComponentId"`
	Peripherals     []libLayoutPeripheral `json:"peripherals,omitempty"`
	NetPolicies     map[string]string     `json:"netPolicies"`
}
type libLayoutSource struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Connectivity  connectivity.Document  `json:"connectivity"`
	Sheet         layoutBBox             `json:"sheet"`
	SheetBorder   *layoutBBox            `json:"sheetBorder,omitempty"`
	Keepouts      []layoutBBox           `json:"keepouts"`
	Measurements  []powerLayoutPlacement `json:"measurements"`
	LayoutModules []libLayoutModule      `json:"layoutModules"`
	MaxCandidates int                    `json:"maxCandidates,omitempty"`
}

func newSchLibLayoutCmd(stdout, stderr io.Writer) *cobra.Command {
	var from, out string
	c := &cobra.Command{Use: "lib-layout", Short: "Calculate Lib placement and wiring offline from canonical nets and measured pins", Long: `Plan translation-only modules from a declared core on a 5-raw grid. Input contains
schemaVersion:1, connectivity, sheet, keepouts, measurements and layoutModules.
Each layoutModule declares id/title/coreComponentId and netPolicies keyed by netId:
direct, local_power, local_ground or module_port. Optional peripherals specify
componentId, optional pinNumber and attachTo:{componentId,pinNumber}. The target
may be a core or another module member; connection data must already agree.
Peripherals may face perpendicular to their reference pin (e.g. shunt capacitors).
Placement follows existing electrical branches; no connectivity or pose is invented. Search is bounded to
400 raw outward distance and 200 raw lateral distance, ordered by shortest connection
length then straightness. Optional maxCandidates limits the total search (default
20000, range 1..1000000). Unresolved input fails
without writing output. Output is a validated source for sch compose. Naming markers are local for power
and ground islands; direct/module_port nets form an actual wire tree.

Examples:
  easyeda sch lib-layout --from layout-input.json --out composition.json
  easyeda sch compose --from composition.json --out plan.json`, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if from == "" {
			return fmt.Errorf("--from is required")
		}
		b, e := os.ReadFile(from)
		if e != nil {
			return e
		}
		src, e := decodeLibLayout(b)
		if e != nil {
			return e
		}
		result, e := planLibLayout(src)
		if e != nil {
			return e
		}
		b, e = json.MarshalIndent(result, "", "  ")
		if e != nil {
			return e
		}
		b = append(b, '\n')
		if out != "" {
			a, _ := filepath.Abs(from)
			z, _ := filepath.Abs(out)
			fi, _ := os.Stat(from)
			fo, _ := os.Stat(out)
			if a == z || (fi != nil && fo != nil && os.SameFile(fi, fo)) {
				return fmt.Errorf("--out must not overwrite the measured input")
			}
			e = os.WriteFile(out, b, 0644)
		} else {
			_, e = stdout.Write(b)
		}
		if e != nil {
			return e
		}
		parts, wires, markers := 0, 0, 0
		for _, module := range result.Modules {
			parts += len(module.Placements)
			wires += len(module.Wires)
			markers += len(module.Flags)
		}
		fmt.Fprintf(stderr, "lib-layout: %d modules, %d parts, %d wires, %d markers; connectivity and measured poses preserved; compose source ready\n", len(result.Modules), parts, wires, markers)
		return nil
	}}
	c.Flags().StringVar(&from, "from", "", "canonical connectivity, measured geometry and module layout intent JSON")
	c.Flags().StringVar(&out, "out", "", "write compose source only after complete validation")
	return c
}

func decodeLibLayout(raw []byte) (libLayoutSource, error) {
	var src libLayoutSource
	if err := connectivity.DecodeStrictDesignJSON(raw, &src); err != nil {
		return src, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return src, err
	}
	if _, err := connectivity.DecodeDesign(fields["connectivity"]); err != nil {
		return src, err
	}
	require := func(object map[string]json.RawMessage, where string, keys ...string) error {
		for _, key := range keys {
			if len(object[key]) == 0 || string(object[key]) == "null" {
				return fmt.Errorf("%s.%s requires explicit measured evidence", where, key)
			}
		}
		return nil
	}
	if err := require(fields, "input", "keepouts", "measurements", "layoutModules", "sheet"); err != nil {
		return src, err
	}
	for _, key := range []string{"sheet", "sheetBorder"} {
		if len(fields[key]) == 0 {
			continue
		}
		var box map[string]json.RawMessage
		_ = json.Unmarshal(fields[key], &box)
		if err := require(box, key, "minX", "minY", "maxX", "maxY"); err != nil {
			return src, err
		}
	}
	var keepouts []map[string]json.RawMessage
	_ = json.Unmarshal(fields["keepouts"], &keepouts)
	for i, box := range keepouts {
		if err := require(box, fmt.Sprintf("keepouts[%d]", i), "minX", "minY", "maxX", "maxY"); err != nil {
			return src, err
		}
	}
	var measurements []map[string]json.RawMessage
	if err := json.Unmarshal(fields["measurements"], &measurements); err != nil {
		return src, err
	}
	for i, m := range measurements {
		where := fmt.Sprintf("measurements[%d]", i)
		if err := require(m, where, "designator", "x", "y", "rotation", "mirror", "bbox", "pins"); err != nil {
			return src, err
		}
		var box map[string]json.RawMessage
		_ = json.Unmarshal(m["bbox"], &box)
		if err := require(box, where+".bbox", "minX", "minY", "maxX", "maxY"); err != nil {
			return src, err
		}
		var pins []map[string]json.RawMessage
		_ = json.Unmarshal(m["pins"], &pins)
		for j, p := range pins {
			if err := require(p, fmt.Sprintf("%s.pins[%d]", where, j), "number", "net", "x", "y"); err != nil {
				return src, err
			}
		}
	}
	return src, nil
}

func libPinSide(p powerLayoutPin, b layoutBBox) (string, error) {
	var found []string
	for _, d := range []string{"left", "right", "up", "down"} {
		if schTerminalPointsOutward(p, b, d) {
			found = append(found, d)
		}
	}
	if len(found) != 1 {
		return "", fmt.Errorf("pin %s has ambiguous/unknown measured outward side", p.Number)
	}
	return found[0], nil
}
func libPin(c powerLayoutPlacement, n string) (powerLayoutPin, bool) {
	for _, p := range c.Pins {
		if p.Number == n {
			return p, true
		}
	}
	return powerLayoutPin{}, false
}

// Bounded route alternatives: straight, then the two one-bend Manhattan paths.
func libRoutes(a, b powerLayoutPin) [][]powerLayoutWire {
	if a.Net == "" || a.Net != b.Net {
		return nil
	}
	if a.X == b.X && a.Y == b.Y {
		return [][]powerLayoutWire{{}}
	}
	wire := func(x, y [2]float64) powerLayoutWire { return powerLayoutWire{Net: a.Net, Points: [][2]float64{x, y}} }
	s, t := [2]float64{a.X, a.Y}, [2]float64{b.X, b.Y}
	if a.X == b.X || a.Y == b.Y {
		return [][]powerLayoutWire{{wire(s, t)}}
	}
	m1, m2 := [2]float64{b.X, a.Y}, [2]float64{a.X, b.Y}
	return [][]powerLayoutWire{{wire(s, m1), wire(m1, t)}, {wire(s, m2), wire(m2, t)}}
}

// Marker leads may branch off the already connected same-net tree.
func libPlaceMarker(p *powerLayoutPlan, q powerLayoutPin, kind string) bool {
	var body layoutBBox
	for _, c := range p.Placements {
		for _, cp := range c.Pins {
			if cp == q {
				body = c.BBox
			}
		}
	}
	side, e := libPinSide(q, body)
	if e != nil {
		return false
	}
	segments, e := schTerminalSegments(p)
	if e != nil {
		return false
	}
	directions := []string{side, "up", "down", "left", "right"}
	if kind == "ground" {
		directions = []string{"down", side, "left", "right", "up"}
	}
	if kind == "power" {
		directions = []string{"up", side, "left", "right", "down"}
	}
	seen := map[string]bool{}
	for _, direction := range directions {
		if seen[direction] {
			continue
		}
		seen[direction] = true
		for offset := 10.0; offset <= 300; offset += 5 {
			f := powerLayoutFlag{Net: q.Net, Kind: kind, PinX: q.X, PinY: q.Y, Direction: direction, Offset: offset}
			if schTerminalCandidate(p, f, segments) == nil {
				candidate := *p
				candidate.Flags = append(append([]powerLayoutFlag(nil), p.Flags...), f)
				if validateLibGeometry(&candidate) == nil {
					p.Flags = candidate.Flags
					return true
				}
			}
		}
	}
	return false
}

// Merge same-net collinear intervals, including a new segment that bridges two
// old ones. Rebuild slices so searching a candidate cannot mutate its parent.
func libAppendRoute(existing, route []powerLayoutWire) []powerLayoutWire {
	out := make([]powerLayoutWire, len(existing))
	for i, w := range existing {
		out[i] = w
		out[i].Points = append([][2]float64(nil), w.Points...)
	}
	for _, w := range route {
		w.Points = append([][2]float64(nil), w.Points...)
		for i := 0; i < len(out); {
			old := out[i]
			if old.Net == w.Net && len(w.Points) == 2 && len(old.Points) == 2 {
				a, b, c, d := w.Points[0], w.Points[1], old.Points[0], old.Points[1]
				horizontal := a[1] == b[1] && a[1] == c[1] && a[1] == d[1]
				vertical := a[0] == b[0] && a[0] == c[0] && a[0] == d[0]
				if (horizontal || vertical) && plSegmentsMeet(a, b, c, d) {
					axis := 0
					if vertical {
						axis = 1
					}
					lo, hi := a, a
					for _, point := range [][2]float64{b, c, d} {
						if point[axis] < lo[axis] {
							lo = point
						}
						if point[axis] > hi[axis] {
							hi = point
						}
					}
					w.Points = [][2]float64{lo, hi}
					out = append(out[:i], out[i+1:]...)
					i = 0 // The extended interval may now reach an earlier segment.
					continue
				}
			}
			i++
		}
		out = append(out, w)
	}
	return out
}
