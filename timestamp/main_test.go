package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestParseInput(t *testing.T) {
	var (
		epoch = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		pst   = time.FixedZone("PST", -8*3600)
		ref   = time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)
	)

	tests := []struct {
		input   string
		want    time.Time
		wantErr bool
	}{
		// rfc3339
		{"2006-01-02T15:04:05Z", ref, false},
		{"2006-01-02T15:04:05-08:00", time.Date(2006, 1, 2, 15, 4, 5, 0, pst), false},

		// additional supported variations
		{"2006-01-02", time.Date(2006, 1, 2, 0, 0, 0, 0, time.UTC), false},
		{"2006-01-02T15:04:05", ref, false},
		{"2006-01-02 15:04:05", ref, false},

		// unix timestamp
		{"1", time.Date(1970, 1, 1, 0, 0, 1, 0, time.UTC), false},
		{"1136214245", ref, false},
		{"11362142450", ref, false}, // sub-second precision
		{"113621424500", ref, false},
		{"1136214245000", ref, false},

		// ordinal dates
		{"2006-002", time.Date(2006, 1, 2, 0, 0, 0, 0, time.UTC), false},
		{"2010-034", time.Date(2010, 2, 3, 0, 0, 0, 0, time.UTC), false},

		// newbase60 epoch days
		{"3eA", time.Date(2006, 1, 2, 0, 0, 0, 0, time.UTC), false},
		{"PL6", time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC), true},

		// zero values
		{"0", epoch, false},  // parsed as unix timestamp
		{"O", epoch, false},  // parsed as newbase60
		{"O0", epoch, false}, // parsed as newbase60
	}

	for _, tt := range tests {
		got, err := parseInput(tt.input, time.UTC)
		if tt.wantErr && err == nil {
			t.Errorf("parseInput(%q) did not return expected error", tt.input)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("parseInput(%q) returned unexpected error: %v", tt.input, err)
		}
		if !got.Equal(tt.want) {
			t.Errorf("parseInput(%q) returned %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseNow(t *testing.T) {
	now := time.Now()
	tm, err := parseInput("", time.UTC)
	if err != nil {
		t.Errorf("parseInput('') returned unexpected error: %v", err)
	}
	if tm.Before(now.Add(-time.Minute)) || tm.After(now.Add(time.Minute)) {
		t.Errorf("parseInput('') returned time outside of now +/- a minute: %v", tm)
	}
}

func TestPrintOutput(t *testing.T) {
	ny, _ := time.LoadLocation("America/New_York")

	tests := []struct {
		time time.Time
		loc  *time.Location
		want []string
	}{
		{
			time.Date(2006, 1, 2, 0, 0, 0, 0, time.UTC),
			nil,
			[]string{"2006-01-02 00:00:00 +0000 UTC"},
		},
		{
			time.Date(2006, 1, 2, 0, 0, 0, 0, ny),
			nil,
			[]string{"2006-01-02 00:00:00 -0500 EST", "RFC 3339:", "RFC 3339 (UTC):"},
		},
	}

	for _, tt := range tests {
		b := new(bytes.Buffer)
		printOutput(b, tt.time, tt.loc)
		for _, w := range tt.want {
			if !strings.Contains(b.String(), w) {
				t.Errorf("printOutput(%v, %v) did not included expected string %q", tt.time, tt.loc, w)
			}
		}
		if len(tt.want) == 0 {
			t.Errorf("printOutput(%v, %v): %q", tt.time, tt.loc, b.String())
		}
	}
}
