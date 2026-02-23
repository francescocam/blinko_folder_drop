package metrics

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type Metrics struct {
	discovered  atomic.Uint64
	processedOK atomic.Uint64
	failed      atomic.Uint64
	retried     atomic.Uint64
	deleted     atomic.Uint64
	movedFailed atomic.Uint64
}

func New() *Metrics { return &Metrics{} }

func (m *Metrics) IncDiscovered()  { m.discovered.Add(1) }
func (m *Metrics) IncProcessedOK() { m.processedOK.Add(1) }
func (m *Metrics) IncFailed()      { m.failed.Add(1) }
func (m *Metrics) IncRetried()     { m.retried.Add(1) }
func (m *Metrics) IncDeleted()     { m.deleted.Add(1) }
func (m *Metrics) IncMovedFailed() { m.movedFailed.Add(1) }

func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = fmt.Fprintf(w,
			"blinko_folder_drop_discovered_total %d\nblinko_folder_drop_processed_ok_total %d\nblinko_folder_drop_failed_total %d\nblinko_folder_drop_retried_total %d\nblinko_folder_drop_deleted_total %d\nblinko_folder_drop_moved_failed_total %d\n",
			m.discovered.Load(),
			m.processedOK.Load(),
			m.failed.Load(),
			m.retried.Load(),
			m.deleted.Load(),
			m.movedFailed.Load(),
		)
	})
}
