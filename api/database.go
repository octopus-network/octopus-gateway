package main

import (
	"fmt"

	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/jmoiron/sqlx"
)

func NewSqlx(cfg *Config) *sqlx.DB {
	dsn := fmt.Sprintf("user=%s password=%s host=%s port=%s database=%s sslmode=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Database,
		cfg.Database.SslMode)
	db, err := sqlx.Connect(cfg.Database.Driver, dsn)
	if err != nil {
		panic(err)
	}

	// db.DB.SetMaxOpenConns(cfg.Database.MaxConnectionPool)
	return db
}
