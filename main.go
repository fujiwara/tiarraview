package tiarraview

import (
	"context"
	"log/slog"
	"time"

	"github.com/alecthomas/kong"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/shogo82148/go-retry"
)

var config = &Config{}

func Run(ctx context.Context) error {
	k := kong.Parse(config)
	switch k.Command() {
	case "server":
		return runServer(ctx)
	case "import":
		return runImport(ctx)
	case "init":
		return runInit(ctx)
	}
	return nil
}

var policy = retry.Policy{
	MinDelay: 100 * time.Millisecond,
	MaxDelay: time.Second,
	MaxCount: 20,
}

func openDB(ctx context.Context) (*sqlx.DB, error) {
	var db *sqlx.DB
	if err := policy.Do(ctx, func() error {
		slog.Info("open DB", "file", config.DBFile)
		var err error
		db, err = sqlx.Open("sqlite3", config.DBFile)
		if err != nil {
			slog.Warn("failed to open DB", "error", err)
			return err
		}
		if err := db.PingContext(ctx); err != nil {
			slog.Warn("failed to ping DB", "error", err)
			db.Close()
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return db, nil
}
