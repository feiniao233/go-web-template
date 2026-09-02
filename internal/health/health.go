package health

import "context"

type Check struct {
	Name string
	Ping func(context.Context) error
}
