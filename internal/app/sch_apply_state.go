package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
)

// These expectations refer to physical part designators and pin numbers, never
// list indexes. Pins is exhaustive per part; optional values select the facts to
// check before placement, before wiring, or after the final topology readback.
type schematicStateExpectation struct {
	ExactParts bool                                `json:"exactParts,omitempty"`
	Parts      map[string]schematicPartExpectation `json:"parts"`
}

type schematicPartExpectation struct {
	PrimitiveID string                             `json:"primitiveId,omitempty"`
	X           *float64                           `json:"x,omitempty"`
	Y           *float64                           `json:"y,omitempty"`
	Rotation    *float64                           `json:"rotation,omitempty"`
	Mirror      *bool                              `json:"mirror,omitempty"`
	Pins        map[string]schematicPinExpectation `json:"pins"`
}

type schematicPinExpectation struct {
	X   *float64 `json:"x,omitempty"`
	Y   *float64 `json:"y,omitempty"`
	Net *string  `json:"net,omitempty"`
}

// Explicit null must not silently mean "do not check this net". Omission is
// useful for geometry-only gates, while null denotes unavailable information.
func (p *schematicPinExpectation) UnmarshalJSON(data []byte) error {
	type plain schematicPinExpectation
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var value plain
	if err := dec.Decode(&value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if net, exists := fields["net"]; exists && bytes.Equal(bytes.TrimSpace(net), []byte("null")) {
		return fmt.Errorf("expectSchematic pin net cannot be null; omit net for a geometry-only gate")
	}
	*p = schematicPinExpectation(value)
	return nil
}

type schematicExpectationError struct{ cause error }

func (e *schematicExpectationError) Error() string { return "expectSchematic: " + e.cause.Error() }
func (e *schematicExpectationError) Unwrap() error { return e.cause }

func validateSchematicExpectationStep(s *playbookStep) error {
	if s.ExpectSchematic == nil {
		return nil
	}
	if s.Action != "schematic.components.list" || s.Payload["includePins"] != true {
		return fmt.Errorf("expectSchematic requires action schematic.components.list with includePins:true")
	}
	return s.ExpectSchematic.validate()
}

func (e *schematicStateExpectation) validate() error {
	if len(e.Parts) == 0 {
		return fmt.Errorf("expectSchematic.parts must not be empty")
	}
	for _, ref := range sortedStateKeys(e.Parts) {
		part := e.Parts[ref]
		if strings.TrimSpace(ref) == "" || part.Pins == nil {
			return fmt.Errorf("expectSchematic part %q requires a designator and exhaustive pins map", ref)
		}
		for field, value := range map[string]*float64{"x": part.X, "y": part.Y, "rotation": part.Rotation} {
			if value != nil && !finiteStateNumber(*value) {
				return fmt.Errorf("%s.%s must be finite", ref, field)
			}
		}
		for _, number := range sortedStateKeys(part.Pins) {
			pin := part.Pins[number]
			if strings.TrimSpace(number) == "" {
				return fmt.Errorf("%s has an empty pin number", ref)
			}
			if pin.Net != nil && strings.TrimSpace(*pin.Net) == "" {
				return fmt.Errorf("%s.%s expected net must not be empty", ref, number)
			}
			for field, value := range map[string]*float64{"x": pin.X, "y": pin.Y} {
				if value != nil && !finiteStateNumber(*value) {
					return fmt.Errorf("%s.%s.%s must be finite", ref, number, field)
				}
			}
		}
	}
	return nil
}

func (e *schematicStateExpectation) jsonValue() any {
	raw, _ := json.Marshal(e)
	var value any
	_ = json.Unmarshal(raw, &value)
	return value
}

func (e *schematicStateExpectation) check(result any, vars map[string]string) error {
	value, err := substVars(e.jsonValue(), vars)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var expected schematicStateExpectation
	if err := json.Unmarshal(raw, &expected); err != nil {
		return err
	}
	if err := expected.validate(); err != nil {
		return err
	}
	root, ok := result.(map[string]any)
	if !ok {
		return fmt.Errorf("missing schematic components result")
	}
	components, ok := root["components"].([]any)
	if !ok {
		return fmt.Errorf("components must be an available array")
	}
	parts := map[string]map[string]any{}
	for _, entry := range components {
		part, ok := entry.(map[string]any)
		if !ok {
			return fmt.Errorf("malformed component record")
		}
		kind, ok := part["componentType"].(string)
		if !ok || kind == "" {
			return fmt.Errorf("component %v has unknown componentType", part["primitiveId"])
		}
		if kind != "part" {
			continue
		}
		ref, ok := part["designator"].(string)
		if !ok || ref == "" {
			return fmt.Errorf("part %v has no designator", part["primitiveId"])
		}
		if _, exists := parts[ref]; exists {
			return fmt.Errorf("duplicate part designator %s; cannot attribute pins unambiguously", ref)
		}
		parts[ref] = part
	}
	if expected.ExactParts {
		for _, ref := range sortedStateKeys(parts) {
			if _, exists := expected.Parts[ref]; !exists {
				return fmt.Errorf("unexpected part %s", ref)
			}
		}
	}
	for _, ref := range sortedStateKeys(expected.Parts) {
		want := expected.Parts[ref]
		have, exists := parts[ref]
		if !exists {
			return fmt.Errorf("missing part %s", ref)
		}
		if want.PrimitiveID != "" && have["primitiveId"] != want.PrimitiveID {
			return fmt.Errorf("%s primitiveId: got %v, want %s", ref, have["primitiveId"], want.PrimitiveID)
		}
		for field, value := range map[string]*float64{"x": want.X, "y": want.Y, "rotation": want.Rotation} {
			if err := compareStateCoordinate(ref, field, have, value); err != nil {
				return err
			}
		}
		if want.Mirror != nil && have["mirror"] != *want.Mirror {
			return fmt.Errorf("%s mirror: got %v, want %v", ref, have["mirror"], *want.Mirror)
		}
		if available, exists := have["pinsAvailable"]; exists && available != true {
			return fmt.Errorf("%s pins unavailable", ref)
		}
		pinList, ok := have["pins"].([]any)
		if !ok {
			return fmt.Errorf("%s pins must be an available array", ref)
		}
		pins := map[string]map[string]any{}
		for _, entry := range pinList {
			pin, ok := entry.(map[string]any)
			if !ok {
				return fmt.Errorf("%s has a malformed pin", ref)
			}
			number, ok := pin["pinNumber"].(string)
			if !ok || number == "" {
				return fmt.Errorf("%s has an unknown pin number", ref)
			}
			if _, exists := pins[number]; exists {
				return fmt.Errorf("%s has duplicate pin number %s", ref, number)
			}
			if _, exists := want.Pins[number]; !exists {
				return fmt.Errorf("%s has unexpected pin %s", ref, number)
			}
			pins[number] = pin
		}
		for _, number := range sortedStateKeys(want.Pins) {
			wantPin := want.Pins[number]
			pin, exists := pins[number]
			if !exists {
				return fmt.Errorf("%s missing pin %s", ref, number)
			}
			for field, value := range map[string]*float64{"x": wantPin.X, "y": wantPin.Y} {
				if err := compareStateCoordinate(ref+"."+number, field, pin, value); err != nil {
					return err
				}
			}
			if wantPin.Net != nil {
				net, known := pin["net"].(string)
				if have["netAmbiguous"] == true || !known {
					return fmt.Errorf("%s.%s net is unknown or ambiguous", ref, number)
				}
				if net != *wantPin.Net {
					return fmt.Errorf("%s.%s net: got %q, want %q", ref, number, net, *wantPin.Net)
				}
			}
		}
	}
	return nil
}

func compareStateCoordinate(ref, field string, have map[string]any, want *float64) error {
	if want == nil {
		return nil
	}
	actual, ok := toFloat(have[field])
	if !ok || !finiteStateNumber(actual) || math.Abs(actual-*want) > 1e-6 {
		return fmt.Errorf("%s.%s: got %v, want %g (tolerance 1e-6)", ref, field, have[field], *want)
	}
	return nil
}

func finiteStateNumber(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }

func sortedStateKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
