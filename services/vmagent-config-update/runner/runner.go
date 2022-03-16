package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type Runner struct {
	fn       func(ctx context.Context) error
	wg       sync.WaitGroup
	syncCh   chan struct{}
	cancelFn context.CancelFunc
}

func New(fn func(ctx context.Context) error) *Runner {
	return &Runner{fn: fn}
}

func (r *Runner) Close() error {
	r.cancelFn()
	close(r.syncCh)
	r.wg.Wait()
	return nil
}

func (r *Runner) Run(interval time.Duration) error {
	if r.syncCh != nil {
		return fmt.Errorf("BUG: checker.Run() has been called multiple times")
	}

	t := time.NewTicker(interval)
	ctx, cancel := context.WithCancel(context.Background())
	if err := r.fn(ctx); err != nil {
		cancel()
		return err
	}

	r.cancelFn = cancel
	r.syncCh = make(chan struct{})

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		for {
			select {
			case <-t.C:
				err := r.fn(ctx)
				if ctx.Err() == context.Canceled {
					logger.Infof("runner has been closed")
					return
				}
				if err != nil {
					logger.Errorf("runner func failed: %s", err)
				}
			case <-r.syncCh:
				return
			}
		}
	}()

	return nil
}
