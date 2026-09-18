package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckWorkflow(t *testing.T) {
	pinned := "0123456789abcdef0123456789abcdef01234567"
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{name: "block pinned", content: "steps:\n  - uses: owner/action@" + pinned + " # v1\n"},
		{name: "flow pinned", content: "steps: [{ name: Checkout, uses: owner/action@" + pinned + " }]\n"},
		{name: "quoted key pinned", content: "steps:\n  - \"uses\": owner/action@" + pinned + "\n"},
		{name: "local action", content: "steps:\n  - uses: ./.github/actions/local\n"},
		{name: "mutable tag", content: "steps:\n  - uses: owner/action@v1\n", wantErr: true},
		{name: "flow mutable tag", content: "steps: [{ name: Checkout, uses: owner/action@v1 }]\n", wantErr: true},
		{name: "multiline flow mutable", content: "steps: [\n  { name: Checkout,\n    uses: owner/action@main }\n]\n", wantErr: true},
		{name: "quoted key mutable", content: "steps:\n  - \"uses\": owner/action@main\n", wantErr: true},
		{name: "alias", content: "action: &mutable owner/action@v1\nsteps:\n  - uses: *mutable\n", wantErr: true},
		{name: "aliased pinned step", content: "shared: &step\n  uses: owner/action@" + pinned + "\nsteps:\n  - *step\n", wantErr: true},
		{name: "aliased uses key", content: "name: &uses uses\nsteps:\n  - *uses: owner/action@main\n", wantErr: true},
		{name: "multiline scalar", content: "steps:\n  - uses: >-\n      owner/action@v1\n", wantErr: true},
		{name: "docker reference", content: "steps:\n  - uses: docker://alpine:latest\n", wantErr: true},
		{name: "reusable workflow pinned", content: "jobs:\n  call:\n    uses: owner/repo/.github/workflows/build.yml@" + pinned + "\n"},
		{name: "script text ignored", content: "steps:\n  - run: |\n      echo 'uses: owner/action@v1'\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workflow.yml")
			if err := os.WriteFile(path, []byte(test.content), 0600); err != nil {
				t.Fatal(err)
			}
			err := checkWorkflow(path)
			if (err != nil) != test.wantErr {
				t.Fatalf("checkWorkflow() error = %v, wantErr %v", err, test.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), path+":") {
				t.Fatalf("error lacks file location: %v", err)
			}
		})
	}
}
