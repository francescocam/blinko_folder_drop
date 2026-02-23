//go:build !windows

package platform

import "context"

func TryRunWindowsService(_ string, _ func(context.Context) error) (bool, error) {
	return false, nil
}
