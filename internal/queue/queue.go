package queue

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"blinko-folder-drop/internal/blinko"
	"blinko-folder-drop/internal/metrics"
	"blinko-folder-drop/internal/processor"
	"blinko-folder-drop/internal/store"
)

type Job struct {
	FilePath string
	FileName string
	Size     int64
	ModTime  time.Time
	Ext      string
	Attempt  int
}

type Queue struct {
	jobs           chan Job
	workers        int
	maxRetries     int
	retryBaseDelay time.Duration
	processor      *processor.Processor
	metrics        *metrics.Metrics
	dedupe         *store.Dedupe
	logf           func(string, ...any)
}

func New(size, workers, maxRetries int, retryBaseDelay time.Duration, processor *processor.Processor, metrics *metrics.Metrics, dedupe *store.Dedupe, logf func(string, ...any)) *Queue {
	return &Queue{
		jobs:           make(chan Job, size),
		workers:        workers,
		maxRetries:     maxRetries,
		retryBaseDelay: retryBaseDelay,
		processor:      processor,
		metrics:        metrics,
		dedupe:         dedupe,
		logf:           logf,
	}
}

func (q *Queue) EnqueuePath(path string) {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	job := Job{
		FilePath: abs,
		FileName: fi.Name(),
		Size:     fi.Size(),
		ModTime:  fi.ModTime(),
		Ext:      filepath.Ext(fi.Name()),
	}
	key := makeKey(job)
	if !q.dedupe.Allow(key) {
		return
	}
	q.metrics.IncDiscovered()
	q.jobs <- job
}

func (q *Queue) Run(ctx context.Context) error {
	for i := 0; i < q.workers; i++ {
		go q.worker(ctx, i+1)
	}
	<-ctx.Done()
	return ctx.Err()
}

func (q *Queue) worker(ctx context.Context, id int) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-q.jobs:
			q.handleJob(ctx, id, job)
		}
	}
}

func (q *Queue) handleJob(ctx context.Context, workerID int, job Job) {
	err := q.processor.Process(ctx, job.FilePath)
	if err == nil {
		deleted, ferr := q.processor.FinalizeSuccess(job.FilePath)
		if ferr != nil {
			q.metrics.IncFailed()
			q.logf("level=error msg=finalize_failed worker=%d file=%q error=%q", workerID, job.FilePath, ferr.Error())
			return
		}
		q.metrics.IncProcessedOK()
		if deleted {
			q.metrics.IncDeleted()
		}
		q.logf("level=info msg=processed worker=%d file=%q", workerID, job.FilePath)
		return
	}

	if isRetryable(err) && job.Attempt < q.maxRetries {
		job.Attempt++
		delay := q.retryBaseDelay * time.Duration(1<<(job.Attempt-1))
		q.metrics.IncRetried()
		q.logf("level=warn msg=retry worker=%d file=%q attempt=%d delay=%s error=%q", workerID, job.FilePath, job.Attempt, delay, err.Error())
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			q.jobs <- job
			return
		}
	}

	q.metrics.IncFailed()
	perm := !isRetryable(err)
	rec := processor.FailureRecord{
		Error:     err.Error(),
		At:        time.Now().UTC(),
		Attempts:  job.Attempt + 1,
		Permanent: perm,
	}
	if qerr := q.processor.Quarantine(job.FilePath, rec); qerr != nil {
		q.logf("level=error msg=quarantine_failed worker=%d file=%q error=%q", workerID, job.FilePath, qerr.Error())
		return
	}
	q.metrics.IncMovedFailed()
	q.logf("level=error msg=quarantined worker=%d file=%q attempts=%d permanent=%t", workerID, job.FilePath, rec.Attempts, rec.Permanent)
}

func makeKey(job Job) string {
	s := fmt.Sprintf("%s|%d|%d", job.FilePath, job.Size, job.ModTime.UnixNano())
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func isRetryable(err error) bool {
	var he *blinko.HTTPError
	if errors.As(err, &he) {
		if he.StatusCode == 429 || he.StatusCode >= 500 {
			return true
		}
		return false
	}
	return true
}
