package tiarraview

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
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

func openDB() (*sqlx.DB, error) {
	db, err := sqlx.Open("sqlite3", config.DBFile)
	if err != nil {
		return nil, err
	}
	return db, nil
}
