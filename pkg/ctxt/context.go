package ctxt

import (
	"context"
	"os"
	"os/signal"
)

// WithSignalTrap returns a context which is cancelled when the given signal(s)
// is received
func WithSignalTrap(ctx context.Context, sig ...os.Signal) context.Context {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		trap := make(chan os.Signal, 1)
		defer close(trap)

		signal.Notify(trap, sig...)
		defer signal.Stop(trap)

		select {
		case <-trap:
			cancel()
		case <-ctx.Done():
		}
	}()

	return ctx
}
