package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/josebianco/go-vue-data-lab/internal/config"
)

func TestParseArgsRejectsMissingCommand(t *testing.T) {
	if _, err := parseArgs(nil, &bytes.Buffer{}); err == nil {
		t.Fatal("expected error when no subcommand is given")
	}
}

func TestParseArgsRejectsUnknownCommand(t *testing.T) {
	if _, err := parseArgs([]string{"sideways"}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

func TestParseArgsRejectsUnknownDB(t *testing.T) {
	_, err := parseArgs([]string{"up", "-db", "both"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for invalid -db value")
	}
	if !strings.Contains(err.Error(), "both") {
		t.Errorf("error %q should mention the invalid value", err)
	}
}

func TestParseArgsRejectsInvalidSteps(t *testing.T) {
	if _, err := parseArgs([]string{"down", "-steps", "0"}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected error for steps=0")
	}
	if _, err := parseArgs([]string{"down", "-steps", "-3"}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected error for negative steps")
	}
}

func TestParseArgsDefaults(t *testing.T) {
	opts, err := parseArgs([]string{"up"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if opts.command != "up" || opts.db != "all" || opts.steps != 1 || opts.dir != "" {
		t.Fatalf("opts = %+v, want up/all/steps=1/empty dir", opts)
	}
}

func TestRunHelpReturnsNil(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"help"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatalf("run help: %v", err)
	}
	if !strings.Contains(out.String(), "migrate up") {
		t.Errorf("usage output should mention subcommands, got %q", out.String())
	}
}

func TestRunDashHReturnsNil(t *testing.T) {
	if err := run([]string{"-h"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run -h: %v", err)
	}
}

func TestBuildTargets(t *testing.T) {
	cfg := &config.Config{DBSourceName: "lab_source", DBTargetName: "lab_target"}

	cases := []struct {
		db      string
		want    []string
		wantDir []string
	}{
		{"all", []string{"source", "target"}, []string{"migrations/source", "migrations/target"}},
		{"source", []string{"source"}, []string{"migrations/source"}},
		{"target", []string{"target"}, []string{"migrations/target"}},
	}

	for _, tc := range cases {
		got := buildTargets(&options{command: "up", db: tc.db, dir: "migrations"}, cfg)
		if len(got) != len(tc.want) {
			t.Fatalf("db=%s: got %d targets, want %d", tc.db, len(got), len(tc.want))
		}
		for i := range got {
			if got[i].label != tc.want[i] {
				t.Errorf("db=%s: target[%d].label = %q, want %q", tc.db, i, got[i].label, tc.want[i])
			}
			if got[i].dir != tc.wantDir[i] {
				t.Errorf("db=%s: target[%d].dir = %q, want %q", tc.db, i, got[i].dir, tc.wantDir[i])
			}
		}
	}
}
