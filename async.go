package flatte

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
