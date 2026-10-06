package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDecodeLogRotation(t *testing.T) {
	tests := []struct {
		name string
		json string
		want LogRotation
	}{
		{
			name: "omitted section defaults",
			json: `{ "log_level": "debug" }`,
			want: LogRotation{IntervalHours: 24, MaxBackups: 5, MaxDayAge: 30, Compress: true},
		},
		{
			name: "zero values use defaults",
			json: `{ "log_rotation": { "interval_hours": 0, "max_backups": 0, "max_day_age": 0 } }`,
			want: LogRotation{IntervalHours: 24, MaxBackups: 5, MaxDayAge: 30, Compress: true},
		},
		{
			name: "custom canonical settings preserve false compression",
			json: `{ "log_rotation": { "interval_hours": 12, "max_backups": 10, "max_day_age": 7, "compress": false } }`,
			want: LogRotation{IntervalHours: 12, MaxBackups: 10, MaxDayAge: 7},
		},
		{
			name: "legacy section remains compatible",
			json: `{ "LogRotation": { "max_mb_size": 10, "max_backups": 3, "max_day_age": 14, "compress": false } }`,
			want: LogRotation{IntervalHours: 24, MaxMBSize: 10, MaxBackups: 3, MaxDayAge: 14},
		},
		{
			name: "canonical section takes precedence",
			json: `{ "LogRotation": { "interval_hours": 12, "compress": false }, "log_rotation": { "interval_hours": 48 } }`,
			want: LogRotation{IntervalHours: 48, MaxBackups: 5, MaxDayAge: 30, Compress: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeConfig([]byte(tt.json))
			if err != nil {
				t.Fatal(err)
			}
			if got.LogRotation != tt.want {
				t.Fatalf("rotation = %+v, want %+v", got.LogRotation, tt.want)
			}
		})
	}
}

func TestLogRotationValidation(t *testing.T) {
	const maxDuration = time.Duration(1<<63 - 1)
	tests := []struct {
		name string
		cfg  LogRotation
		want string
	}{
		{name: "negative interval", cfg: LogRotation{IntervalHours: -1}, want: "interval_hours"},
		{name: "negative count", cfg: LogRotation{MaxBackups: -1}, want: "max_backups"},
		{name: "negative age", cfg: LogRotation{MaxDayAge: -1}, want: "max_day_age"},
		{name: "interval overflow", cfg: LogRotation{IntervalHours: int(maxDuration/time.Hour) + 1}, want: "interval_hours"},
		{name: "age overflow", cfg: LogRotation{MaxDayAge: int(maxDuration/(24*time.Hour)) + 1}, want: "max_day_age"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.cfg.WithDefaults()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want error containing %q", err, tt.want)
			}
		})
	}
}

func TestLoadAndReloadApplyDefaultsAndRejectInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	write := func(contents string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	m := NewManager(path)
	write(`{ "log_level": "debug", "log_rotation": { "interval_hours": 12, "compress": false } }`)
	if err := m.Load(); err != nil {
		t.Fatal(err)
	}
	if got := m.Get().LogRotation; got.IntervalHours != 12 || got.Compress {
		t.Fatalf("loaded rotation = %+v", got)
	}
	write(`{ "log_level": "info" }`)
	if err := m.Reload(); err != nil {
		t.Fatal(err)
	}
	want := LogRotation{IntervalHours: 24, MaxBackups: 5, MaxDayAge: 30, Compress: true}
	if got := m.Get().LogRotation; got != want {
		t.Fatalf("reloaded rotation = %+v, want %+v", got, want)
	}
	write(`{ "log_level": "trace", "log_rotation": { "interval_hours": -1 } }`)
	for _, load := range []func() error{m.Load, m.Reload} {
		if err := load(); err == nil {
			t.Fatal("invalid rotation was accepted")
		}
		if got := m.Get(); got.LogLevel != "info" || got.LogRotation != want {
			t.Fatalf("invalid load changed current config: %+v", got)
		}
	}
}

func TestSaveUsesCanonicalRotationAndPreservesSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	contents := `{ "LogRotation": { "max_mb_size": 10, "compress": false }, "production": { "last_production": 42 } }`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	m := NewManager(path)
	if err := m.Load(); err != nil {
		t.Fatal(err)
	}
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		t.Fatal(err)
	}
	if _, exists := fields["LogRotation"]; exists {
		t.Fatal("saved deprecated LogRotation property")
	}
	if _, exists := fields["log_rotation"]; !exists {
		t.Fatal("missing canonical log_rotation property")
	}
	if err := m.Reload(); err != nil {
		t.Fatal(err)
	}
	got := m.Get()
	if got.LogRotation.Compress || got.LogRotation.MaxMBSize != 10 || got.Production.LastProduction != 42 {
		t.Fatalf("save/reload did not preserve settings: %+v", got)
	}
}
