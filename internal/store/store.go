package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"quicklink/internal/cache"
	"quicklink/internal/database"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	ErrNotFound      = errors.New("short url not found")
	ErrAlreadyExists = errors.New("short code or alias already exists")
)

type URLRecord struct {
	Code         string     `gorm:"primaryKey;size:64" json:"code"`
	OriginalURL  string     `gorm:"type:text;not null" json:"original_url"`
	ShortURL     string     `gorm:"-" json:"short_url,omitempty"`
	Clicks       int64      `gorm:"default:0;not null" json:"clicks"`
	CreatedAt    time.Time  `gorm:"index:,sort:desc;not null" json:"created_at"`
	LastAccessed *time.Time `json:"last_accessed,omitempty"`
}

func (URLRecord) TableName() string {
	return "urls"
}

type Store interface {
	Save(ctx context.Context, record *URLRecord) error
	Get(ctx context.Context, code string) (*URLRecord, error)
	GetOriginalURL(ctx context.Context, code string) (string, error)
	IncrementClicks(ctx context.Context, code string) error
	List(ctx context.Context) ([]*URLRecord, error)
	Delete(ctx context.Context, code string) error
	Count(ctx context.Context) (int, error)
	TotalClicks(ctx context.Context) (int64, error)
}

type gormStoreOptions struct {
	redis       *cache.Client
	autoMigrate bool
	cacheTTL    time.Duration
}

type GormStoreOption func(*gormStoreOptions)

func WithCache(c *cache.Client) GormStoreOption {
	return func(o *gormStoreOptions) {
		o.redis = c
	}
}

func WithAutoMigrate(enable bool) GormStoreOption {
	return func(o *gormStoreOptions) {
		o.autoMigrate = enable
	}
}

func WithCacheTTL(ttl time.Duration) GormStoreOption {
	return func(o *gormStoreOptions) {
		if ttl > 0 {
			o.cacheTTL = ttl
		}
	}
}

type GormStore struct {
	db       *gorm.DB
	redis    *cache.Client
	cacheTTL time.Duration
}

func NewGormStore(db *database.DB, opts ...GormStoreOption) (*GormStore, error) {
	o := &gormStoreOptions{
		autoMigrate: true,
		cacheTTL:    24 * time.Hour,
	}

	for _, opt := range opts {
		opt(o)
	}

	if o.autoMigrate {
		if err := db.AutoMigrate(&URLRecord{}); err != nil {
			return nil, err
		}
	}

	return &GormStore{
		db:       db.DB,
		redis:    o.redis,
		cacheTTL: o.cacheTTL,
	}, nil
}

func (s *GormStore) Save(ctx context.Context, record *URLRecord) error {
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}

	err := s.db.WithContext(ctx).Create(record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) ||
			strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
			strings.Contains(strings.ToLower(err.Error()), "unique") {
			return ErrAlreadyExists
		}
		return err
	}

	if s.redis != nil {
		_ = s.redis.Set(ctx, record.Code, record.OriginalURL, s.cacheTTL)
	}

	return nil
}

func (s *GormStore) Get(ctx context.Context, code string) (*URLRecord, error) {
	var rec URLRecord
	err := s.db.WithContext(ctx).First(&rec, "code = ?", code).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &rec, nil
}

func (s *GormStore) GetOriginalURL(ctx context.Context, code string) (string, error) {
	if s.redis != nil {
		cachedURL, err := s.redis.Get(ctx, code)
		if err == nil && cachedURL != "" {
			return cachedURL, nil
		} else if err != nil && !errors.Is(err, redis.Nil) {
		}
	}

	rec, err := s.Get(ctx, code)
	if err != nil {
		return "", err
	}

	if s.redis != nil {
		_ = s.redis.Set(ctx, code, rec.OriginalURL, s.cacheTTL)
	}

	return rec.OriginalURL, nil
}

func (s *GormStore) IncrementClicks(ctx context.Context, code string) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Model(&URLRecord{}).
		Where("code = ?", code).
		Updates(map[string]any{
			"clicks":        gorm.Expr("clicks + ?", 1),
			"last_accessed": &now,
		}).Error
}

func (s *GormStore) List(ctx context.Context) ([]*URLRecord, error) {
	var list []*URLRecord
	err := s.db.WithContext(ctx).Order("created_at desc").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (s *GormStore) Delete(ctx context.Context, code string) error {
	res := s.db.WithContext(ctx).Delete(&URLRecord{}, "code = ?", code)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}

	if s.redis != nil {
		_ = s.redis.Del(ctx, code)
	}

	return nil
}

func (s *GormStore) Count(ctx context.Context) (int, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&URLRecord{}).Count(&count).Error
	return int(count), err
}

func (s *GormStore) TotalClicks(ctx context.Context) (int64, error) {
	var total sql.NullInt64
	err := s.db.WithContext(ctx).Model(&URLRecord{}).Select("COALESCE(SUM(clicks), 0)").Scan(&total).Error
	if err != nil {
		return 0, err
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Int64, nil
}
