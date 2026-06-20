// Package repository provides database access to conversation data via sqlx and SQLite.
package repository

import (
	"github.com/adrg/xdg"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

// ConnectDB connects to SQLite, runs migrations, and sets the database path based on the isDev flag.
func ConnectDB(isDev bool) (*sqlx.DB, error) {
	var dbPath string
	if !isDev {
		var err error
		dbPath, err = xdg.DataFile("akernel/akernel.db")
		if err != nil {
			return nil, err
		}
	} else {
		dbPath = "./database/database.db"
	}

	db, err := sqlx.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	goose.SetBaseFS(nil)

	if err = goose.SetDialect("sqlite3"); err != nil {
		return nil, err
	}

	if err = goose.Up(db.DB, "migrations"); err != nil {
		return nil, err
	}

	if _, err = db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, err
	}

	return db, nil
}
