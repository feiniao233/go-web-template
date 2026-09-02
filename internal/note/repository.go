package note

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var (
	ErrNotFound    = errors.New("note not found")
	ErrUnavailable = errors.New("database is unavailable")
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, page, pageSize int) ([]Note, int64, error) {
	if r.db == nil {
		return nil, 0, ErrUnavailable
	}
	db := r.db.WithContext(ctx)
	var total int64
	if err := db.Model(&Note{}).Count(&total).Error; err != nil {
		return nil, 0, unavailable(err)
	}
	notes := []Note{}
	if err := db.Order("id").Offset((page - 1) * pageSize).Limit(pageSize).Find(&notes).Error; err != nil {
		return nil, 0, unavailable(err)
	}
	return notes, total, nil
}

func (r *Repository) Get(ctx context.Context, id int64) (Note, error) {
	if r.db == nil {
		return Note{}, ErrUnavailable
	}
	var n Note
	err := r.db.WithContext(ctx).First(&n, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Note{}, ErrNotFound
	}
	if err != nil {
		return Note{}, unavailable(err)
	}
	return n, nil
}

func (r *Repository) Create(ctx context.Context, values Values) (Note, error) {
	if r.db == nil {
		return Note{}, ErrUnavailable
	}
	n := Note{Title: values.Title, Content: values.Content}
	if err := r.db.WithContext(ctx).Create(&n).Error; err != nil {
		return Note{}, unavailable(err)
	}
	return n, nil
}

func (r *Repository) Update(ctx context.Context, id int64, values Values) (Note, error) {
	if r.db == nil {
		return Note{}, ErrUnavailable
	}
	db := r.db.WithContext(ctx)
	result := db.Model(&Note{}).Where("id = ?", id).Updates(map[string]any{"title": values.Title, "content": values.Content})
	if result.Error != nil {
		return Note{}, unavailable(result.Error)
	}
	if result.RowsAffected == 0 {
		return Note{}, ErrNotFound
	}
	var n Note
	if err := db.First(&n, id).Error; err != nil {
		return Note{}, unavailable(err)
	}
	return n, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	if r.db == nil {
		return ErrUnavailable
	}
	result := r.db.WithContext(ctx).Delete(&Note{}, id)
	if result.Error != nil {
		return unavailable(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func unavailable(err error) error { return fmt.Errorf("%w: %w", ErrUnavailable, err) }
