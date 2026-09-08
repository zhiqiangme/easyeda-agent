package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocGuardRecoversMissingActivePCB(t *testing.T) {
	for _, tc := range []struct {
		name, lookupUUID, lookupProject, selector string
		listFails, lookupFails, wantErr           bool
	}{
		{"empty enumeration", "pcb1", "project1", "PCB1", false, false, false},
		{"failed enumeration", "pcb1", "project1", "PCB1", true, false, false},
		{"uuid selector", "pcb1", "project1", "pcb1", false, false, false},
		{"other target", "pcb1", "project1", "PCB2", false, false, true},
		{"lookup failed", "pcb1", "project1", "PCB1", false, true, true},
		{"tab drift", "pcb2", "project1", "PCB1", false, false, true},
		{"project drift", "pcb1", "project2", "PCB1", false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/health" {
					_, _ = w.Write([]byte(`{"service":"easyeda-agent","windows":[{"windowId":"w1"}]}`))
					return
				}
				var req struct {
					Action   string `json:"action"`
					WindowID string `json:"windowId"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
					return
				}
				calls = append(calls, req.Action)
				ctx := map[string]any{"documentUuid": "pcb1", "documentType": "pcb", "projectUuid": "project1"}
				result := map[string]any{}
				ok := true
				switch req.Action {
				case "document.current":
					result["uuid"] = "pcb1"
				case "schematic.pages.list":
					result["pages"] = []any{}
				case "pcb.documents.list":
					result["pcbs"] = []any{}
					ok = !tc.listFails
				case "pcb.board.info":
					ctx["documentUuid"], ctx["projectUuid"] = tc.lookupUUID, tc.lookupProject
					result["pcb"] = map[string]any{"uuid": tc.lookupUUID, "name": "PCB1"}
					ok = !tc.lookupFails
				default:
					t.Errorf("unexpected action %s", req.Action)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": ok, "result": result, "context": ctx, "error": map[string]any{"code": "EDA_CALL_FAILED", "message": "fixture failure"}})
			}))
			defer srv.Close()
			hostPort := strings.TrimPrefix(srv.URL, "http://")
			host, port, _ := strings.Cut(hostPort, ":")
			cfg := &appConfig{host: host, ports: port + "-" + port, doc: tc.selector}
			err := ensureActiveDoc(cfg, "w1")
			if (err != nil) != tc.wantErr {
				t.Fatalf("error %v, wantErr %v", err, tc.wantErr)
			}
			if cfg.doc != tc.selector {
				t.Fatal("discovery mutated the caller's target guard")
			}
			if len(calls) != 4 || calls[3] != "pcb.board.info" {
				t.Fatalf("unexpected recovery calls: %v", calls)
			}
		})
	}
}
