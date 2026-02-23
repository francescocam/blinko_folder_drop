package watcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Event struct {
	Path string
}

type Config struct {
	InputDir  string
	Recursive bool
	StableFor time.Duration
	ScanEvery time.Duration
}

type Watcher struct {
	cfg Config
}

func New(cfg Config) *Watcher { return &Watcher{cfg: cfg} }

func (w *Watcher) Run(ctx context.Context, out chan<- Event, logf func(string, ...any)) error {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fw.Close()

	if err := w.addWatchPaths(fw); err != nil {
		return err
	}

	scanAndEmit := func() {
		_ = filepath.WalkDir(w.cfg.InputDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if !w.cfg.Recursive && path != w.cfg.InputDir {
					return filepath.SkipDir
				}
				return nil
			}
			if stable, _ := IsStable(path, w.cfg.StableFor); stable {
				out <- Event{Path: path}
			}
			return nil
		})
	}

	scanAndEmit()
	ticker := time.NewTicker(w.cfg.ScanEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			scanAndEmit()
		case ev, ok := <-fw.Events:
			if !ok {
				return errors.New("watcher events channel closed")
			}
			if ev.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename) == 0 {
				continue
			}
			fi, err := os.Stat(ev.Name)
			if err == nil && fi.IsDir() && w.cfg.Recursive {
				_ = fw.Add(ev.Name)
				continue
			}
			if stable, _ := IsStable(ev.Name, w.cfg.StableFor); stable {
				out <- Event{Path: ev.Name}
			}
		case err, ok := <-fw.Errors:
			if !ok {
				return errors.New("watcher error channel closed")
			}
			logf("watcher error: %v", err)
		}
	}
}

func (w *Watcher) addWatchPaths(fw *fsnotify.Watcher) error {
	if !w.cfg.Recursive {
		return fw.Add(w.cfg.InputDir)
	}
	return filepath.WalkDir(w.cfg.InputDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return fw.Add(path)
		}
		return nil
	})
}

func IsStable(path string, stableFor time.Duration) (bool, error) {
	first, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	time.Sleep(stableFor)
	second, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if first.Size() != second.Size() {
		return false, nil
	}
	if !first.ModTime().Equal(second.ModTime()) {
		return false, nil
	}
	return true, nil
}
