package logger

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dronm/elkonexport/config"
)

func TestInitializeDefaults(t *testing.T) {
	defer FlushLogFile()
	if err := Initialize("info", filepath.Join(t.TempDir(), "application.log")); err != nil {
		t.Fatal(err)
	}
	w := logOutput.(*timedWriter)
	if w.interval != 24*time.Hour || w.options.MaxBackups != 5 || w.options.MaxDayAge != 30 || !w.options.Compress {
		t.Fatalf("unexpected defaults: %+v", w.options)
	}
	Logger.Info("default logging works")
	FlushLogFile()
	select {
	case <-w.done:
	default:
		t.Fatal("FlushLogFile returned before scheduler stopped")
	}
}

func TestInitializeSamePathClosesOldWriterBeforeRotation(t *testing.T) {
	defer FlushLogFile()
	path := filepath.Join(t.TempDir(), "application.log")
	options := config.LogRotation{Compress: false}
	if err := Initialize("info", path, options); err != nil {
		t.Fatal(err)
	}
	Logger.Info("old record")
	old := logOutput.(*timedWriter)
	stale := time.Now().UTC().Truncate(24 * time.Hour).Add(-24 * time.Hour)
	if err := os.Chtimes(path, stale, stale); err != nil {
		t.Fatal(err)
	}
	if err := Initialize("info", path, options); err != nil {
		t.Fatal(err)
	}
	select {
	case <-old.done:
	default:
		t.Fatal("old scheduler still running")
	}
	Logger.Info("new record")
	active, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) == 0 {
		t.Fatal("new logger did not write to active path")
	}
	archive := filepath.Join(filepath.Dir(path), "application-"+stale.Format(archiveTimeFormat)+".log")
	archived, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if string(archived) == string(active) {
		t.Fatal("new record was written into old archive")
	}
	if old.file != nil || !old.closed {
		t.Fatal("old writer still owns an open file")
	}
}

func TestInitializeFailureLeavesConsoleLogger(t *testing.T) {
	defer FlushLogFile()
	if err := Initialize("info", filepath.Join(t.TempDir(), "application.log")); err != nil {
		t.Fatal(err)
	}
	if err := Initialize("info", t.TempDir()); err == nil {
		t.Fatal("directory was accepted as a log file")
	}
	if Logger.Out != os.Stderr || logOutput != nil {
		t.Fatal("failed reinitialization left a closed output attached")
	}
}
