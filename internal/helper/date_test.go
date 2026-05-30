// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package helper

import (
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "RFC3339 format",
			value: "2024-01-15T10:30:00Z",
			want:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:  "date only format",
			value: "2024-01-15",
			want:  time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "empty string returns zero",
			value:   "",
			want:    time.Time{},
			wantErr: false,
		},
		{
			name:    "whitespace returns zero",
			value:   "   ",
			want:    time.Time{},
			wantErr: false,
		},
		{
			name:    "invalid format",
			value:   "invalid-date",
			want:    time.Time{},
			wantErr: true,
		},
		{
			name:  "RFC3339 with timezone",
			value: "2024-01-15T10:30:00+08:00",
			want:  time.Date(2024, 1, 15, 2, 30, 0, 0, time.UTC),
		},
		{
			name:  "RFC3339 with milliseconds",
			value: "2024-01-15T10:30:00.123Z",
			want:  time.Date(2024, 1, 15, 10, 30, 0, 123000000, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDate(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !got.Equal(tt.want) {
				t.Errorf("ParseDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeDateRange(t *testing.T) {
	tests := []struct {
		name      string
		startRaw  string
		endRaw    string
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			name:      "normal range",
			startRaw:  "2024-01-01",
			endRaw:    "2024-01-31",
			wantStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), // End expanded to full day
		},
		{
			name:      "reversed range swaps dates",
			startRaw:  "2024-01-31",
			endRaw:    "2024-01-01",
			wantStart: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), // After swap: end expanded first to 2024-01-02
			wantEnd:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "empty strings return zero",
			startRaw:  "",
			endRaw:    "",
			wantStart: time.Time{},
			wantEnd:   time.Time{},
		},
		{
			name:      "only start date",
			startRaw:  "2024-01-15",
			endRaw:    "",
			wantStart: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Time{},
		},
		{
			name:      "only end date",
			startRaw:  "",
			endRaw:    "2024-01-15",
			wantStart: time.Time{},
			wantEnd:   time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC), // Expanded to full day
		},
		{
			name:      "RFC3339 timestamps",
			startRaw:  "2024-01-15T10:00:00Z",
			endRaw:    "2024-01-15T18:00:00Z",
			wantStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2024, 1, 15, 18, 0, 0, 0, time.UTC), // Not expanded, has time
		},
		{
			name:      "datetime range with midnight end gets expanded (no swap needed)",
			startRaw:  "2024-01-15T10:00:00Z",
			endRaw:    "2024-01-15T00:00:00Z",
			wantStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), // Original start
			wantEnd:   time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),  // End expanded to next day
		},
		{
			name:     "invalid start date",
			startRaw: "invalid",
			endRaw:   "2024-01-15",
			wantErr:  true,
		},
		{
			name:     "invalid end date",
			startRaw: "2024-01-15",
			endRaw:   "invalid",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd, err := NormalizeDateRange(tt.startRaw, tt.endRaw)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeDateRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !gotStart.Equal(tt.wantStart) {
				t.Errorf("NormalizeDateRange() start = %v, want %v", gotStart, tt.wantStart)
			}
			if !gotEnd.Equal(tt.wantEnd) {
				t.Errorf("NormalizeDateRange() end = %v, want %v", gotEnd, tt.wantEnd)
			}
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{
			name: "valid timestamp",
			t:    time.Date(2024, 1, 15, 10, 30, 45, 123000000, time.UTC),
			want: "2024-01-15T10:30:45Z",
		},
		{
			name: "zero timestamp returns empty",
			t:    time.Time{},
			want: "",
		},
		{
			name: "timestamp with nanoseconds",
			t:    time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC),
			want: "2024-01-15T10:30:45Z", // Nanoseconds truncated in RFC3339
		},
		{
			name: "midnight timestamp",
			t:    time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			want: "2024-01-15T00:00:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTimestamp(tt.t)
			if got != tt.want {
				t.Errorf("FormatTimestamp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatTimestampPtr(t *testing.T) {
	tests := []struct {
		name string
		t    *time.Time
		want string
	}{
		{
			name: "valid pointer",
			t:    func() *time.Time { t := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC); return &t }(),
			want: "2024-01-15T10:30:00Z",
		},
		{
			name: "nil pointer returns empty",
			t:    nil,
			want: "",
		},
		{
			name: "pointer to zero time",
			t:    func() *time.Time { t := time.Time{}; return &t }(),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTimestampPtr(tt.t)
			if got != tt.want {
				t.Errorf("FormatTimestampPtr() = %v, want %v", got, tt.want)
			}
		})
	}
}
