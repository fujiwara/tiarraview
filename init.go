package tiarraview

import (
	"context"
	_ "embed"
	"log/slog"
	"os"
)

//go:embed db/schema.sql
var embeddedSchema []byte

func runInit(ctx context.Context) error {
	err := os.Remove(config.DBFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := openDB(ctx)
	if err != nil {
		return err
	}
	var sc []byte
	if config.SchemaFile != "" {
		slog.Info("loading schema", "file", config.SchemaFile)
		b, err := os.ReadFile(config.SchemaFile)
		if err != nil {
			return err
		}
		sc = b
	} else {
		sc = embeddedSchema
	}
	slog.Info("executing schema", "content", string(sc))
	_, err = db.Exec(string(sc))
	if err != nil {
		return err
	}
	slog.Info("schema loaded")
	return nil
}
