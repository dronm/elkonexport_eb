package logger

import (
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dronm/elkonexport/config"
)

func rotationOptions() config.LogRotation {
	return config.LogRotation{IntervalHours: 24, MaxBackups: 20, MaxDayAge: 30}
}

func testArchivePath(filename string, period time.Time) string {
	ext := filepath.Ext(filename)
	return strings.TrimSuffix(filename, ext) + "-" + period.UTC().Format(archiveTimeFormat) + ext
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s content = %q, want %q", path, got, want)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected %s to be absent, got %v", path, err)
	}
}

func writeLog(t *testing.T, w *timedWriter, content string) {
	t.Helper()
	n, err := w.Write([]byte(content))
	if err != nil || n != len(content) {
		t.Fatalf("write = (%d, %v), want (%d, nil)", n, err, len(content))
	}
}

func TestTimedWriterRotatesAtExactUTCDayBoundary(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "application.log")
	zone := time.FixedZone("UTC+03", 3*60*60)
	now := time.Date(2026, 10, 6, 15, 0, 0, 0, zone)
	w, err := openTimedWriter(filename, rotationOptions(), func() time.Time { return now }, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	period := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	if !w.periodStart.Equal(period) {
		t.Fatalf("initial period = %s, want %s", w.periodStart, period)
	}
	writeLog(t, w, "first\n")
	now = time.Date(2026, 10, 7, 2, 59, 59, 999999999, zone)
	writeLog(t, w, "last\n")
	assertMissing(t, testArchivePath(filename, period))
	now = now.Add(time.Nanosecond)
	writeLog(t, w, "next\n")
	if delta := w.periodStart.Sub(period); delta != 24*time.Hour {
		t.Fatalf("period advanced by %s, want exactly 24h", delta)
	}
	assertFileContent(t, testArchivePath(filename, period), "first\nlast\n")
	assertFileContent(t, filename, "next\n")
}

func TestTimedWriterSupportsConfiguredInterval(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "application.log")
	now := time.Date(2026, 10, 6, 11, 59, 59, 0, time.UTC)
	options := rotationOptions()
	options.IntervalHours = 12
	w, err := openTimedWriter(filename, options, func() time.Time { return now }, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	period := w.periodStart
	writeLog(t, w, "morning\n")
	now = now.Add(time.Second)
	writeLog(t, w, "afternoon\n")
	assertFileContent(t, testArchivePath(filename, period), "morning\n")
	assertFileContent(t, filename, "afternoon\n")
	if w.periodStart.Sub(period) != 12*time.Hour {
		t.Fatalf("configured period advanced by %s", w.periodStart.Sub(period))
	}
}

func TestTimedWriterRestartAppendsWithinSameDay(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "application.log")
	now := time.Date(2026, 10, 6, 8, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	w, err := openTimedWriter(filename, rotationOptions(), clock, false)
	if err != nil {
		t.Fatal(err)
	}
	writeLog(t, w, "before restart\n")
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filename, now, now); err != nil {
		t.Fatal(err)
	}
	now = now.Add(8 * time.Hour)
	w, err = openTimedWriter(filename, rotationOptions(), clock, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	writeLog(t, w, "after restart\n")
	assertFileContent(t, filename, "before restart\nafter restart\n")
	assertMissing(t, testArchivePath(filename, now.Truncate(24*time.Hour)))
}

func TestTimedWriterArchivesStaleLogOnStartup(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "application.log")
	now := time.Date(2026, 10, 6, 8, 0, 0, 0, time.UTC)
	previous := now.Add(-48 * time.Hour)
	writeTestFile(t, filename, "previous run\n")
	if err := os.Chtimes(filename, previous, previous); err != nil {
		t.Fatal(err)
	}
	w, err := openTimedWriter(filename, rotationOptions(), func() time.Time { return now }, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	assertFileContent(t, testArchivePath(filename, previous.Truncate(24*time.Hour)), "previous run\n")
	assertFileContent(t, filename, "")
	if !w.periodStart.Equal(now.Truncate(24 * time.Hour)) {
		t.Fatalf("stale log left current period at %s", w.periodStart)
	}
	writeLog(t, w, "current run\n")
	assertFileContent(t, filename, "current run\n")
}

func TestTimedWriterDoesNotArchiveEmptyPeriods(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "application.log")
	now := time.Date(2026, 10, 6, 8, 0, 0, 0, time.UTC)
	w, err := openTimedWriter(filename, rotationOptions(), func() time.Time { return now }, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	previous := w.periodStart
	now = now.Add(72 * time.Hour)
	writeLog(t, w, "first entry\n")
	assertMissing(t, testArchivePath(filename, previous))
	assertFileContent(t, filename, "first entry\n")
}

func TestTimedWriterRetentionDeletesOnlyRecognizedRegularArchives(t *testing.T) {
	for _, tc := range []struct {
		name         string
		maxBackups   int
		maxAge       int
		retainMiddle bool
	}{
		{name: "count limit", maxBackups: 2, maxAge: 30},
		{name: "age limit", maxBackups: 20, maxAge: 3, retainMiddle: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			filename := filepath.Join(directory, "application.audit.log")
			now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
			recent := testArchivePath(filename, now.Add(-24*time.Hour))
			second := strings.TrimSuffix(recent, ".log") + "-1.log.gz"
			middle := testArchivePath(filename, now.Add(-48*time.Hour))
			old := testArchivePath(filename, now.Add(-5*24*time.Hour))
			legacy := filepath.Join(directory, "application.audit-2026-09-20T00-00-00.000.log.gz")
			for _, path := range []string{recent, second, middle, old, legacy} {
				writeTestFile(t, path, filepath.Base(path))
			}
			unrelated := []string{
				filepath.Join(directory, "other-2026-09-20T00-00-00.000000000Z.log"),
				filepath.Join(directory, "applicationXaudit-2026-09-20T00-00-00.000000000Z.log"),
				filepath.Join(directory, "application.audit-2026-99-20T00-00-00.000000000Z.log"),
				filepath.Join(directory, "application.audit-2026-09-20T00-00-00.000000000Z-0.log"),
				filepath.Join(directory, "application.audit-2026-09-20T00-00-00.000000000Z.log.bak"),
			}
			for _, path := range unrelated {
				writeTestFile(t, path, "preserve")
			}
			archiveDirectory := testArchivePath(filename, now.Add(-10*24*time.Hour))
			if err := os.Mkdir(archiveDirectory, 0o700); err != nil {
				t.Fatal(err)
			}
			linkedArchive := testArchivePath(filename, now.Add(-11*24*time.Hour))
			linkCreated := os.Symlink(unrelated[0], linkedArchive) == nil
			options := rotationOptions()
			options.MaxBackups = tc.maxBackups
			options.MaxDayAge = tc.maxAge
			w, err := openTimedWriter(filename, options, func() time.Time { return now }, false)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = w.Close() })
			assertFileContent(t, recent, filepath.Base(recent))
			assertFileContent(t, second, filepath.Base(second))
			if tc.retainMiddle {
				assertFileContent(t, middle, filepath.Base(middle))
			} else {
				assertMissing(t, middle)
			}
			assertMissing(t, old)
			assertMissing(t, legacy)
			for _, path := range unrelated {
				assertFileContent(t, path, "preserve")
			}
			if info, err := os.Lstat(archiveDirectory); err != nil || !info.IsDir() {
				t.Fatalf("matching directory removed: %v", err)
			}
			if linkCreated {
				if info, err := os.Lstat(linkedArchive); err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("matching symlink removed: %v", err)
				}
			}
		})
	}
}

func TestTimedWriterCompressesCompletedArchive(t *testing.T) {
	for _, name := range []string{"application.log", "application.gz"} {
		t.Run(name, func(t *testing.T) {
			testTimedWriterCompressesCompletedArchive(t, name)
		})
	}
}

func testTimedWriterCompressesCompletedArchive(t *testing.T, name string) {
	t.Helper()
	filename := filepath.Join(t.TempDir(), name)
	now := time.Date(2026, 10, 6, 8, 0, 0, 0, time.UTC)
	options := rotationOptions()
	options.Compress = true
	w, err := openTimedWriter(filename, options, func() time.Time { return now }, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	period := w.periodStart
	content := strings.Repeat("entry with UTF-8: стекло\n", 100)
	writeLog(t, w, content)
	now = now.Add(24 * time.Hour)
	writeLog(t, w, "today\n")
	archive := testArchivePath(filename, period)
	assertMissing(t, archive)
	file, err := os.Open(archive + ".gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf("decompressed archive does not contain all original log entries")
	}
	assertFileContent(t, filename, "today\n")
	entries, err := filepath.Glob(filepath.Join(filepath.Dir(filename), ".log-compress-*"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("compression temporary files remain: %v (%v)", entries, err)
	}
}

func TestTimedWriterArchiveCollisionsPreserveExistingFiles(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "application.log")
	now := time.Date(2026, 10, 6, 8, 0, 0, 0, time.UTC)
	w, err := openTimedWriter(filename, rotationOptions(), func() time.Time { return now }, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	archive := testArchivePath(filename, w.periodStart)
	base := strings.TrimSuffix(archive, ".log")
	writeTestFile(t, archive, "existing plain")
	writeTestFile(t, base+"-1.log.gz", "existing compressed")
	if err := os.Mkdir(base+"-2.log", 0o700); err != nil {
		t.Fatal(err)
	}
	writeLog(t, w, "new archive\n")
	now = now.Add(24 * time.Hour)
	writeLog(t, w, "new day\n")
	assertFileContent(t, archive, "existing plain")
	assertFileContent(t, base+"-1.log.gz", "existing compressed")
	if info, err := os.Stat(base + "-2.log"); err != nil || !info.IsDir() {
		t.Fatalf("existing directory was overwritten: %v", err)
	}
	assertFileContent(t, base+"-3.log", "new archive\n")
	assertFileContent(t, filename, "new day\n")
}

func TestCompressArchiveFailurePreservesSource(t *testing.T) {
	t.Run("existing destination", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "application.log")
		writeTestFile(t, path, "original content")
		writeTestFile(t, path+".gz", "existing destination")
		if err := compressArchive(path); err == nil {
			t.Fatal("compression succeeded despite an existing destination")
		}
		assertFileContent(t, path, "original content")
		assertFileContent(t, path+".gz", "existing destination")
	})
	t.Run("read failure", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "application.log")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := compressArchive(path); err == nil {
			t.Fatal("compression succeeded while reading a directory")
		}
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Fatalf("source removed after compression failure: %v", err)
		}
		assertMissing(t, path+".gz")
		entries, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".log-compress-*"))
		if err != nil || len(entries) != 0 {
			t.Fatalf("temporary files remain after failure: %v (%v)", entries, err)
		}
	})
}

func TestTimedWriterRotationFailurePreservesLogAndCanRecover(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "application.log")
	now := time.Date(2026, 10, 6, 8, 0, 0, 0, time.UTC)
	w, err := openTimedWriter(filename, rotationOptions(), func() time.Time { return now }, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	period := w.periodStart
	writeLog(t, w, "preserve this log\n")
	// Simulate an unavailable source directory without relying on permission
	// failures, which privileged users can bypass.
	w.filename = filepath.Join(filepath.Dir(filename), "unavailable", "application.log")
	now = now.Add(24 * time.Hour)
	if _, err := w.Write([]byte("failed entry\n")); err == nil {
		t.Fatal("rotation succeeded while its configured path was unavailable")
	}
	assertFileContent(t, filename, "preserve this log\n")
	w.filename = filename
	writeLog(t, w, "recovered\n")
	assertFileContent(t, testArchivePath(filename, period), "preserve this log\n")
	assertFileContent(t, filename, "recovered\n")
}

func TestTimedWriterKeepsWritingIfArchiveNameIsTooLong(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, strings.Repeat("x", 230)+".log")
	now := time.Date(2026, 10, 6, 8, 0, 0, 0, time.UTC)
	w, err := openTimedWriter(filename, rotationOptions(), func() time.Time { return now }, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	previous := w.periodStart
	writeLog(t, w, "before boundary\n")
	now = now.Add(24 * time.Hour)
	writeLog(t, w, "preserved after failed rotation\n")
	assertFileContent(t, filename, "before boundary\npreserved after failed rotation\n")
	if !w.periodStart.Equal(previous) {
		t.Fatalf("unsuccessful rotation advanced period to %s", w.periodStart)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filename, previous, previous); err != nil {
		t.Fatal(err)
	}
	recoveredFilename := filepath.Join(directory, "application.log")
	if err := os.Rename(filename, recoveredFilename); err != nil {
		t.Fatal(err)
	}
	recovered, err := openTimedWriter(recoveredFilename, rotationOptions(), func() time.Time { return now }, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recovered.Close() })
	writeLog(t, recovered, "recovered\n")
	assertFileContent(t, testArchivePath(recoveredFilename, previous), "before boundary\npreserved after failed rotation\n")
	assertFileContent(t, recoveredFilename, "recovered\n")
}

func TestTimedWriterSchedulerRotatesWithoutWrites(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "application.log")
	period := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	beforeBoundary := period.Add(24*time.Hour - 75*time.Millisecond)
	writeTestFile(t, filename, "idle application\n")
	if err := os.Chtimes(filename, beforeBoundary, beforeBoundary); err != nil {
		t.Fatal(err)
	}
	var clock atomic.Int64
	clock.Store(beforeBoundary.UnixNano())
	var reads atomic.Int64
	schedulerReady := make(chan struct{})
	advance := make(chan struct{})
	now := func() time.Time {
		current := time.Unix(0, clock.Load()).UTC()
		// Construction reads the clock three times. Hold the scheduler's
		// first read until the test advances the clock, then let it create
		// a short timer using the original pre-midnight reading.
		if reads.Add(1) == 4 {
			close(schedulerReady)
			<-advance
		}
		return current
	}
	w, err := openTimedWriter(filename, rotationOptions(), now, true)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	select {
	case <-schedulerReady:
	case <-time.After(3 * time.Second):
		close(advance)
		t.Fatal("scheduler did not start")
	}
	clock.Store(period.Add(24 * time.Hour).UnixNano())
	close(advance)
	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("idle log was not rotated at its UTC boundary")
		case <-ticker.C:
			if _, err := os.Stat(testArchivePath(filename, period)); err == nil {
				w.mu.Lock()
				w.mu.Unlock()
				assertFileContent(t, testArchivePath(filename, period), "idle application\n")
				assertFileContent(t, filename, "")
				return
			}
		}
	}
}

func TestTimedWriterConcurrentWriteAndClose(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "application.log")
	w, err := openTimedWriter(filename, rotationOptions(), time.Now, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	writeLog(t, w, "entry\n")
	var written atomic.Int64
	written.Store(1)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for worker := 0; worker < 12; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < 100; i++ {
				n, err := w.Write([]byte("entry\n"))
				if errors.Is(err, os.ErrClosed) {
					return
				}
				if err != nil || n != len("entry\n") {
					t.Errorf("concurrent write = (%d, %v)", n, err)
					return
				}
				written.Add(1)
			}
		}()
	}
	for closer := 0; closer < 4; closer++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := w.Close(); err != nil {
				t.Errorf("concurrent close: %v", err)
			}
		}()
	}
	close(start)
	finished := make(chan struct{})
	go func() {
		wg.Wait()
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("concurrent close/write did not finish")
	}
	if _, err := w.Write([]byte("after close\n")); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("write after close = %v, want os.ErrClosed", err)
	}
	assertFileContent(t, filename, strings.Repeat("entry\n", int(written.Load())))
}
