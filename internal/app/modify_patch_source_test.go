package app

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestModifyPatchFile(t *testing.T) {
	for _, domain := range []string{"sch", "pcb"} {
		for _, tc := range []struct {
			name, content string
			extra         []string
			wantErr       bool
		}{
			{"utf8", `{"rotation":90}`, nil, false},
			{"powershell-bom", "\xef\xbb\xbf{\"rotation\":90}", nil, false},
			{"null", "null", nil, true},
			{"array", "[]", nil, true},
			{"empty", "", nil, true},
			{"invalid", "{rotation:90}", nil, true},
			{"utf16", "\xff\xfe{\x00}\x00", nil, true},
			{"conflict", `{"rotation":90}`, []string{"--patch", `{"x":1}`}, true},
		} {
			t.Run(domain+"/"+tc.name, func(t *testing.T) {
				cfg, captured, cleanup := newCapturingDaemon(t)
				defer cleanup()
				path := filepath.Join(t.TempDir(), "补丁 file.json")
				if err := os.WriteFile(path, []byte(tc.content), 0600); err != nil {
					t.Fatal(err)
				}
				var cmd *cobra.Command
				if domain == "sch" {
					cmd = newSchCmd(cfg, io.Discard, io.Discard)
				} else {
					cmd = newPcbCmd(cfg, io.Discard, io.Discard)
				}
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)
				cmd.SetArgs(append([]string{"modify", "--id", "p1", "--patch-file", path}, tc.extra...))
				err := cmd.Execute()
				if (err != nil) != tc.wantErr {
					t.Fatalf("error = %v, wantErr %v", err, tc.wantErr)
				}
				captured.mu.Lock()
				defer captured.mu.Unlock()
				if tc.wantErr {
					if captured.action != "" {
						t.Fatalf("invalid input dispatched %s", captured.action)
					}
				} else {
					patch, ok := captured.payload["patch"].(map[string]any)
					if !ok || patch["rotation"] != float64(90) {
						t.Fatalf("wrong payload: %#v", captured.payload)
					}
				}
			})
		}
	}
}
