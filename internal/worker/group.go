package worker

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"
)

type Func func(context.Context) error

type namedWorker struct {
	name string
	run  Func
}

type Group struct {
	workers []namedWorker
}

func New() *Group { return &Group{} }

func (g *Group) Add(name string, run Func) {
	if name == "" || run == nil {
		panic("worker name and function are required")
	}
	g.workers = append(g.workers, namedWorker{name: name, run: run})
}

func (g *Group) Run(ctx context.Context) error {
	if len(g.workers) == 0 {
		<-ctx.Done()
		return nil
	}
	group, ctx := errgroup.WithContext(ctx)
	for _, item := range g.workers {
		item := item
		group.Go(func() error {
			if err := item.run(ctx); err != nil {
				if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
					return nil
				}
				return fmt.Errorf("worker %s: %w", item.name, err)
			}
			return nil
		})
	}
	return group.Wait()
}
