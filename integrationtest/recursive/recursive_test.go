package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/terraform-linters/tflint/formatter"
)

func TestIntegration(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		dir         string
		error       bool
		ignoreOrder bool
	}{
		{
			name:    "recursive",
			command: "tflint --recursive --format json --force",
			dir:     "basic",
		},
		{
			name:    "recursive + filter",
			command: "tflint --recursive --filter=main.tf --format json --force",
			dir:     "filter",
		},
		{
			name:        "recursive with errors",
			command:     "tflint --recursive --format json --force",
			dir:         "errors",
			error:       true,
			ignoreOrder: true,
		},
		{
			name:    "recursive + chdir",
			command: "tflint --chdir=subdir1 --recursive --format json --force",
			dir:     "chdir",
		},
	}

	dir, _ := os.Getwd()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testDir := filepath.Join(dir, test.dir)
			t.Chdir(testDir)

			args := strings.Split(test.command, " ")
			var cmd *exec.Cmd
			if runtime.GOOS == "windows" {
				cmd = exec.Command("tflint.exe", args[1:]...)
			} else {
				cmd = exec.Command("tflint", args[1:]...)
			}
			outStream, errStream := new(bytes.Buffer), new(bytes.Buffer)
			cmd.Stdout = outStream
			cmd.Stderr = errStream

			if err := cmd.Run(); err != nil && !test.error {
				t.Fatalf("Failed to exec command: %s", err)
			}

			var b []byte
			var err error
			if runtime.GOOS == "windows" && IsWindowsResultExist() {
				b, err = os.ReadFile(filepath.Join(testDir, "result_windows.json"))
			} else {
				b, err = os.ReadFile(filepath.Join(testDir, "result.json"))
			}
			if err != nil {
				t.Fatal(err)
			}

			var expected *formatter.JSONOutput
			if err := json.Unmarshal(b, &expected); err != nil {
				t.Fatal(err)
			}

			var got *formatter.JSONOutput
			if err := json.Unmarshal(outStream.Bytes(), &got); err != nil {
				t.Fatal(err)
			}

			opts := []cmp.Option{
				cmpopts.IgnoreFields(formatter.JSONRule{}, "Link"),
				// Only compare error messages up to the double new line.
				// After this, stderr will be printed which is verbose.
				cmp.Transformer("TruncateMessage", func(e formatter.JSONError) formatter.JSONError {
					if parts := strings.Split(e.Message, "\n\n"); len(parts) > 1 {
						e.Message = parts[0]
					}
					return e
				}),
			}
			if test.ignoreOrder {
				opts = append(opts, cmpopts.SortSlices(func(a, b formatter.JSONError) bool {
					return a.Message > b.Message
				}))
			}
			if diff := cmp.Diff(got, expected, opts...); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func IsWindowsResultExist() bool {
	_, err := os.Stat("result_windows.json")
	return !os.IsNotExist(err)
}
