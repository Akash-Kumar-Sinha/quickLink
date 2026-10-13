package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type options struct {
	maxOpenConns    int
	maxIdleConns    int
	connMaxLifetime time.Duration
	logLevel        logger.LogLevel
}

type Option func(*options)

func WithMaxOpenConns(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.maxOpenConns = n
		}
	}
}

func WithMaxIdleConns(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.maxIdleConns = n
		}
	}
}

func WithConnMaxLifetime(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.connMaxLifetime = d
		}
	}
}

func WithLogLevel(level logger.LogLevel) Option {
	return func(o *options) {
		o.logLevel = level
	}
}

type DB struct {
	*gorm.DB
}

func NewPostgres(databaseURL string, opts ...Option) (*DB, error) {
	o := &options{
		maxOpenConns:    25,
		maxIdleConns:    10,
		connMaxLifetime: 5 * time.Minute,
		logLevel:        logger.Silent,
	}

	for _, opt := range opts {
		opt(o)
	}

	gormDB, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(o.logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database via gorm: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get generic database object: %w", err)
	}

	sqlDB.SetMaxOpenConns(o.maxOpenConns)
	sqlDB.SetMaxIdleConns(o.maxIdleConns)
	sqlDB.SetConnMaxLifetime(o.connMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{gormDB}, nil
}

func (d *DB) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
