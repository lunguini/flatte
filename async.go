package flatte

import "context"

type StateUpdate[S any] interface {
	Apply(*S)
	Name() string
}

type named[S any] struct {
	name  string
	apply func(*S)
}

func Named[S any](name string, apply func(*S)) StateUpdate[S] {
	return named[S]{name: name, apply: apply}
}

func (n named[S]) Name() string { return n.name }
func (n named[S]) Apply(s *S)   { n.apply(s) }

// Async runs work off-loop and sends one named update. Deprecated: use Go,
// which obtains the context and update channel from Effects, or use
// Effects.Send for a custom StateUpdate.
func Async[S, T any](
	ctx context.Context,
	updates chan<- StateUpdate[S],
	spawn func(func()),
	name string,
	work func(context.Context) (T, error),
	fold func(*S, T, error),
) {
	run := func() {
		value, err := work(ctx)
		if ctx.Err() != nil {
			return
		}

		update := Named(name, func(s *S) {
			fold(s, value, err)
		})
		select {
		case updates <- update:
		case <-ctx.Done():
		}
	}
	if spawn != nil {
		spawn(run)
		return
	}
	go run()
}
