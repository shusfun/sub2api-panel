//go:build !linux

package panelupdate

import (
	"context"
	"errors"
)

func RunExecutor() error { return errors.New("更新执行器需要 Linux systemd") }

// The executor is Linux-only. Portable tests exercise state replacement.
func syncDir(string) error { return nil }

func runCommand(context.Context, string, ...string) error {
	return errors.New("发布证明校验需要 Linux systemd 更新执行器")
}
