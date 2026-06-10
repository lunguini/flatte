package flatcore

type Tracer interface {
	Event(Event)
	Update(string)
}

type NoopTracer struct{}

func (NoopTracer) Event(Event)   {}
func (NoopTracer) Update(string) {}

type UpdateTracer func(string)

func (UpdateTracer) Event(Event) {}
func (f UpdateTracer) Update(name string) {
	f(name)
}

func ApplyUpdate[S any](s *S, tracer Tracer, update StateUpdate[S]) {
	if tracer == nil {
		tracer = NoopTracer{}
	}
	tracer.Update(update.Name())
	update.Apply(s)
}
