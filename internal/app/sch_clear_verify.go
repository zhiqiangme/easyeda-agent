package app

import "fmt"

// Missing enumeration classes or warnings are unknown state, never an empty page.
func verifySchClearResult(result map[string]any, requireEmpty bool) error {
	if v, ok := result["warnings"]; ok {
		warnings, valid := v.([]any)
		if !valid || len(warnings) > 0 {
			return fmt.Errorf("page clear enumeration/deletion warnings: %v", v)
		}
	}
	groups, ok := result["deletedIds"].(map[string]any)
	if !ok {
		return fmt.Errorf("missing complete page primitive enumeration")
	}
	// The connector omits empty classes; a successful enumeration is signalled
	// by no warnings. Every reported class must still be a real array.
	for key, ids := range groups {
		if _, ok := ids.([]any); !ok {
			return fmt.Errorf("page enumeration unavailable for %s", key)
		}
	}
	remaining, ok := toFloat(result["remaining"])
	if !ok || !finiteStateNumber(remaining) || remaining < 0 {
		return fmt.Errorf("page remaining count unavailable")
	}
	if requireEmpty && remaining != 0 {
		return fmt.Errorf("page is not empty: %g non-preserved primitives remain", remaining)
	}
	return nil
}
