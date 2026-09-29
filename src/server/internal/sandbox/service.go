package sandbox

import (
	"context"
	"errors"
	"fmt"
)

type service struct{ d Deps }

func (s *service) Exec(ctx context.Context, tenant, execution string, cmd Command) (Output, error) {
	id, err := s.d.Store.Get(ctx, execution)
	if errors.Is(err, ErrNotFound) {
		id, err = s.d.Runtime.Start(ctx, tenant, execution)
		if err != nil {
			return Output{}, fmt.Errorf("start sandbox for %s: %w", execution, err)
		}
		if err := s.d.Store.Put(ctx, execution, id); err != nil {
			return Output{}, fmt.Errorf("remember sandbox for %s: %w", execution, err)
		}
	} else if err != nil {
		return Output{}, err
	}
	out, err := s.d.Runtime.Exec(ctx, id, cmd)
	if err != nil {
		return Output{}, fmt.Errorf("exec in sandbox %s: %w", id, err)
	}
	return out, nil
}

func (s *service) Release(ctx context.Context, execution string) error {
	id, err := s.d.Store.Get(ctx, execution)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := s.d.Runtime.Stop(ctx, id); err != nil {
		return fmt.Errorf("stop sandbox %s: %w", id, err)
	}
	return s.d.Store.Delete(ctx, execution)
}
