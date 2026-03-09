package readiness

import "context"

type NoopPinger struct {
	name string
}

func NewNoopPinger(name string) NoopPinger {
	return NoopPinger{name: name}
}

func (p NoopPinger) Name() string {
	return p.name
}

func (p NoopPinger) Ping(_ context.Context) error {
	return nil
}
