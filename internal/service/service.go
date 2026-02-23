package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"blinko-folder-drop/internal/blinko"
	"blinko-folder-drop/internal/config"
	"blinko-folder-drop/internal/metrics"
	"blinko-folder-drop/internal/processor"
	"blinko-folder-drop/internal/queue"
	"blinko-folder-drop/internal/store"
	"blinko-folder-drop/internal/watcher"
)

type Service struct {
	cfg     config.Config
	metrics *metrics.Metrics
}

func New(cfg config.Config) (*Service, error) {
	return &Service{cfg: cfg, metrics: metrics.New()}, nil
}

func (s *Service) Run(ctx context.Context) error {
	logger := func(format string, args ...any) {
		entry := map[string]any{"ts": time.Now().UTC().Format(time.RFC3339Nano), "msg": fmt.Sprintf(format, args...)}
		b, _ := json.Marshal(entry)
		log.Print(string(b))
	}

	hc := &http.Client{Timeout: s.cfg.HTTP.Timeout}
	client := blinko.New(s.cfg.Blinko.BaseURL, s.cfg.Blinko.JWTToken, hc)
	proc := processor.New(client, processor.Config{
		DeleteOnOK: s.cfg.Processing.DeleteOnOK,
		ArchiveDir: s.cfg.Processing.ArchiveDir,
		FailedDir:  s.cfg.Watch.FailedDir,
	})
	q := queue.New(
		s.cfg.Processing.QueueSize,
		s.cfg.Processing.Workers,
		s.cfg.Processing.MaxRetries,
		s.cfg.Processing.RetryBaseDelay,
		proc,
		s.metrics,
		store.NewDedupe(s.cfg.Watch.StableFor*3),
		logger,
	)

	if s.cfg.Metrics.Enabled {
		mux := http.NewServeMux()
		mux.Handle("/metrics", s.metrics.Handler())
		srv := &http.Server{Addr: s.cfg.Metrics.ListenAddr, Handler: mux}
		go func() {
			logger("level=info msg=metrics_listen addr=%q", s.cfg.Metrics.ListenAddr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger("level=error msg=metrics_server error=%q", err.Error())
			}
		}()
		go func() {
			<-ctx.Done()
			_ = srv.Shutdown(context.Background())
		}()
	}

	events := make(chan watcher.Event, s.cfg.Processing.QueueSize)
	w := watcher.New(watcher.Config{
		InputDir:  s.cfg.Watch.InputDir,
		Recursive: s.cfg.Watch.Recursive,
		StableFor: s.cfg.Watch.StableFor,
		ScanEvery: s.cfg.Watch.ScanEvery,
	})

	go func() {
		if err := w.Run(ctx, events, logger); err != nil && err != context.Canceled {
			logger("level=error msg=watcher_terminated error=%q", err.Error())
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-events:
				if shouldIgnorePath(s.cfg, ev.Path) {
					continue
				}
				q.EnqueuePath(ev.Path)
			}
		}
	}()

	logger("level=info msg=service_started input_dir=%q", s.cfg.Watch.InputDir)
	err := q.Run(ctx)
	if err == context.Canceled {
		return nil
	}
	return err
}

func shouldIgnorePath(cfg config.Config, p string) bool {
	for _, pref := range []string{cfg.Watch.FailedDir, cfg.Processing.ArchiveDir} {
		if pref == "" {
			continue
		}
		rel, err := filepath.Rel(pref, p)
		if err == nil && rel != "." && rel != "" && !strings.HasPrefix(rel, "..") {
			return true
		}
		if p == pref {
			return true
		}
	}
	if len(p) > 11 && p[len(p)-11:] == ".error.json" {
		return true
	}
	return false
}
