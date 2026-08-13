package main

import (
	"flag"
	"reflect"
	"testing"
	"time"
)

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "empty", input: "", want: nil},
		{name: "trims and skips empty values", input: " db:5432, ,cache:6379, ", want: []string{"db:5432", "cache:6379"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitCSV(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("splitCSV(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

func TestHeaderFlags(t *testing.T) {
	var headers headerFlags
	if err := headers.Set(" Authorization: Bearer token "); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if got := headers["Authorization"]; got != "Bearer token" {
		t.Fatalf("Authorization header = %q, want %q", got, "Bearer token")
	}
	if err := headers.Set("invalid"); err == nil {
		t.Fatal("Set accepted an invalid header")
	}
}

func TestBuildChecks(t *testing.T) {
	checks, err := buildChecks(
		"db:5432,cache:6379",
		"http://api:8080/readyz",
		204,
		map[string]string{"X-Ready": "yes"},
		"redis:6379",
		"postgres:5432",
		"/var/run/app.ready",
	)
	if err != nil {
		t.Fatalf("buildChecks returned error: %v", err)
	}

	want := []string{
		"tcp://db:5432",
		"tcp://cache:6379",
		"http://api:8080/readyz",
		"redis://redis:6379",
		"postgres://postgres:5432",
		"file:///var/run/app.ready",
	}
	got := make([]string, len(checks))
	for i, check := range checks {
		got[i] = check.String()
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("check labels = %#v, want %#v", got, want)
	}
	if checks[2].ExpectStatus != 204 || checks[2].Headers["X-Ready"] != "yes" {
		t.Fatalf("HTTP check options = %#v, want status 204 and X-Ready header", checks[2])
	}
}

func TestBuildChecksRequiresAtLeastOneCheck(t *testing.T) {
	if _, err := buildChecks("", "", 200, nil, "", "", ""); err == nil {
		t.Fatal("buildChecks returned nil error for empty input")
	}
}

func TestRemainingCommand(t *testing.T) {
	original := flag.CommandLine
	t.Cleanup(func() { flag.CommandLine = original })

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{name: "no command", args: nil, want: nil},
		{name: "strips separator", args: []string{"--", "/app/server", "--config", "/config.yaml"}, want: []string{"/app/server", "--config", "/config.yaml"}},
		{name: "accepts command without separator", args: []string{"/app/server"}, want: []string{"/app/server"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			if err := fs.Parse(tt.args); err != nil {
				t.Fatalf("parse args: %v", err)
			}
			flag.CommandLine = fs

			if got := remainingCommand(); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("remainingCommand() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestRootContextTimeout(t *testing.T) {
	ctx, cancel := rootContext(1)
	defer cancel()

	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("rootContext did not expire")
	}
}
