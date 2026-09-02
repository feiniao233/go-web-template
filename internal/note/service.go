package note

import "context"

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service { return &Service{repository: repository} }

func (s *Service) List(ctx context.Context, page, pageSize int) ([]Note, int64, error) {
	return s.repository.List(ctx, page, pageSize)
}

func (s *Service) Get(ctx context.Context, id int64) (Note, error) {
	return s.repository.Get(ctx, id)
}

func (s *Service) Create(ctx context.Context, values Values) (Note, error) {
	return s.repository.Create(ctx, values)
}

func (s *Service) Update(ctx context.Context, id int64, values Values) (Note, error) {
	return s.repository.Update(ctx, id, values)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repository.Delete(ctx, id)
}
