//go:build windows

package platform

import (
	"context"
	"fmt"

	"golang.org/x/sys/windows/svc"
)

type runnerFn func(context.Context) error

type svcRunner struct {
	version string
	runner  runnerFn
}

func (s *svcRunner) Execute(_ []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	status <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = s.runner(ctx)
	}()
	current := svc.Status{State: svc.Running, Accepts: accepted}
	status <- current
	for c := range req {
		switch c.Cmd {
		case svc.Interrogate:
			// SCM periodically checks service state via interrogate.
			status <- current
		case svc.Stop, svc.Shutdown:
			current = svc.Status{State: svc.StopPending}
			status <- current
			cancel()
			return false, 0
		default:
		}
	}
	return false, 0
}

func TryRunWindowsService(version string, runner func(context.Context) error) (bool, error) {
	isSvc, err := svc.IsWindowsService()
	if err != nil {
		return false, err
	}
	if !isSvc {
		return false, nil
	}
	if err := svc.Run("blinko-folder-drop", &svcRunner{version: version, runner: runner}); err != nil {
		return true, fmt.Errorf("svc.Run: %w", err)
	}
	return true, nil
}
