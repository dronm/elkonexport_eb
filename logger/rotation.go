package logger

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dronm/elkonexport/config"
)

const (
	archiveTimeFormat = "2006-01-02T15-04-05.000000000Z"
	legacyTimeFormat  = "2006-01-02T15-04-05.000"
	maintenancePeriod = time.Minute
)

// timedWriter keeps one active file and archives completed UTC time periods.
// One application instance must own each configured log filename.
type timedWriter struct {
	mu           sync.Mutex
	filename     string
	file         *os.File
	periodStart  time.Time
	interval     time.Duration
	options      config.LogRotation
	archiveMatch *regexp.Regexp
	legacyMatch  *regexp.Regexp
	closed       bool
	stop         chan struct{}
	done         chan struct{}
	now          func() time.Time
}

func newTimedWriter(filename string, options config.LogRotation) (*timedWriter, error) {
	return openTimedWriter(filename, options, time.Now, true)
}

// The clock and scheduler switch let tests exercise real files without waiting a day.
func openTimedWriter(filename string, options config.LogRotation, now func() time.Time, schedule bool) (*timedWriter, error) {
	absolute, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	ext := filepath.Ext(absolute)
	base := strings.TrimSuffix(filepath.Base(absolute), ext)
	prefix := regexp.QuoteMeta(base) + "-"
	suffix := regexp.QuoteMeta(ext) + `(\.gz)?$`
	w := &timedWriter{
		filename:     absolute,
		interval:     time.Duration(options.IntervalHours) * time.Hour,
		options:      options,
		archiveMatch: regexp.MustCompile(`^` + prefix + `(\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}\.\d{9}Z)(?:-([1-9]\d*))?` + suffix),
		legacyMatch:  regexp.MustCompile(`^` + prefix + `(\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}\.\d{3})` + suffix),
		stop:         make(chan struct{}),
		done:         make(chan struct{}),
		now:          now,
	}
	if info, err := os.Lstat(absolute); err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("log path must be a regular file: %s", absolute)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := w.openFile(); err != nil {
		return nil, err
	}
	info, err := w.file.Stat()
	if err != nil {
		_ = w.file.Close()
		return nil, err
	}
	w.periodStart = now().UTC().Truncate(w.interval)
	if info.Size() > 0 {
		w.periodStart = info.ModTime().UTC().Truncate(w.interval)
	}
	if err := w.rotateIfNeeded(now()); err != nil {
		if w.file != nil {
			_ = w.file.Close()
		}
		return nil, err
	}
	w.report(w.cleanup(now()))
	if schedule {
		go w.run()
	} else {
		close(w.done)
	}
	return w, nil
}

func (w *timedWriter) openFile() error {
	file, err := os.OpenFile(w.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	w.file = file
	return nil
}

func (w *timedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, os.ErrClosed
	}
	if err := w.rotateIfNeeded(w.now()); err != nil {
		if w.file == nil {
			return 0, err
		}
		// Preserve records in the original file while a failed rotation is retried.
		w.report(err)
	}
	return w.file.Write(p)
}

func (w *timedWriter) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		<-w.done
		return nil
	}
	w.closed = true
	close(w.stop)
	var err error
	if w.file != nil {
		err = w.file.Close()
		w.file = nil
	}
	w.mu.Unlock()
	<-w.done
	return err
}

func (w *timedWriter) run() {
	defer close(w.done)
	for {
		now := w.now()
		delay := now.UTC().Truncate(w.interval).Add(w.interval).Sub(now)
		if delay <= 0 || delay > maintenancePeriod {
			delay = maintenancePeriod
		}
		timer := time.NewTimer(delay)
		select {
		case <-w.stop:
			timer.Stop()
			return
		case <-timer.C:
			w.mu.Lock()
			if !w.closed {
				now = w.now()
				w.report(w.rotateIfNeeded(now))
				w.report(w.cleanup(now))
			}
			w.mu.Unlock()
		}
	}
}

// Caller holds mu, or is constructing the writer before publishing it.
func (w *timedWriter) rotateIfNeeded(now time.Time) error {
	if w.file == nil {
		if err := w.openFile(); err != nil {
			return err
		}
	}
	period := now.UTC().Truncate(w.interval)
	if period.Equal(w.periodStart) {
		return nil
	}
	info, err := w.file.Stat()
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		w.periodStart = period
		return nil
	}
	archive, err := w.availableArchive()
	if err != nil {
		return err
	}
	if err := w.file.Close(); err != nil {
		w.file = nil
		return err
	}
	w.file = nil
	if err := os.Rename(w.filename, archive); err != nil {
		// Reopen the original so a later rotation attempt can recover.
		return errors.Join(fmt.Errorf("archive log file: %w", err), w.openFile())
	}
	w.periodStart = period
	err = w.openFile()
	w.report(w.cleanup(now))
	return err
}

func (w *timedWriter) availableArchive() (string, error) {
	ext := filepath.Ext(w.filename)
	base := strings.TrimSuffix(w.filename, ext) + "-" + w.periodStart.Format(archiveTimeFormat)
	for sequence := 0; ; sequence++ {
		name := base
		if sequence > 0 {
			name += "-" + strconv.Itoa(sequence)
		}
		name += ext
		available := true
		for _, candidate := range []string{name, name + ".gz"} {
			_, err := os.Lstat(candidate)
			if err == nil {
				available = false
				break
			}
			if !errors.Is(err, os.ErrNotExist) {
				return "", err
			}
		}
		if available {
			return name, nil
		}
	}
}

type logArchive struct {
	name       string
	time       time.Time
	sequence   int
	compressed bool
}

func (w *timedWriter) parseArchive(name string) (logArchive, bool) {
	if match := w.archiveMatch.FindStringSubmatch(name); match != nil {
		timestamp, err := time.Parse(archiveTimeFormat, match[1])
		if err != nil {
			return logArchive{}, false
		}
		sequence := 0
		if match[2] != "" {
			sequence, err = strconv.Atoi(match[2])
			if err != nil {
				return logArchive{}, false
			}
		}
		return logArchive{name: name, time: timestamp, sequence: sequence, compressed: match[3] != ""}, true
	}
	if match := w.legacyMatch.FindStringSubmatch(name); match != nil {
		timestamp, err := time.ParseInLocation(legacyTimeFormat, match[1], time.Local)
		return logArchive{name: name, time: timestamp, compressed: match[2] != ""}, err == nil
	}
	return logArchive{}, false
}

func (w *timedWriter) cleanup(now time.Time) error {
	entries, err := os.ReadDir(filepath.Dir(w.filename))
	if err != nil {
		return err
	}
	var archives []logArchive
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		if archive, ok := w.parseArchive(entry.Name()); ok {
			archives = append(archives, archive)
		}
	}
	sort.Slice(archives, func(i, j int) bool {
		if archives[i].time.Equal(archives[j].time) {
			if archives[i].sequence == archives[j].sequence {
				return archives[i].name > archives[j].name
			}
			return archives[i].sequence > archives[j].sequence
		}
		return archives[i].time.After(archives[j].time)
	})
	cutoff := now.Add(-time.Duration(w.options.MaxDayAge) * 24 * time.Hour)
	var cleanupErr error
	for i, archive := range archives {
		path := filepath.Join(filepath.Dir(w.filename), archive.name)
		if i >= w.options.MaxBackups || archive.time.Before(cutoff) {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove old log %s: %w", archive.name, err))
			}
		} else if w.options.Compress && !archive.compressed {
			if err := compressArchive(path); err != nil {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("compress log %s: %w", archive.name, err))
			}
		}
	}
	return cleanupErr
}

// Publish a completed gzip before deleting its source. Failures retain the source.
func compressArchive(path string) error {
	if _, err := os.Lstat(path + ".gz"); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return fmt.Errorf("compressed archive already exists")
		}
		return err
	}
	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.CreateTemp(filepath.Dir(path), ".log-compress-*")
	if err != nil {
		return err
	}
	defer os.Remove(target.Name())
	defer target.Close()
	compressed := gzip.NewWriter(target)
	_, copyErr := io.Copy(compressed, source)
	if err := errors.Join(copyErr, compressed.Close()); err != nil {
		return err
	}
	if err := target.Sync(); err != nil {
		return err
	}
	if err := target.Close(); err != nil {
		return err
	}
	if err := source.Close(); err != nil {
		return err
	}
	if err := os.Rename(target.Name(), path+".gz"); err != nil {
		return err
	}
	return os.Remove(path)
}

func (w *timedWriter) report(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "log rotation: %v\n", err)
	}
}
