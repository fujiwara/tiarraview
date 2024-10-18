package tiarraview

import (
	"context"
	"log/slog"
	"os"
)

func runInit(_ context.Context) error {
	err := os.Remove(config.DBFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := openDB()
	if err != nil {
		return err
	}
	slog.Info("loading schema", "file", config.SchemaFile)
	b, err := os.ReadFile(config.SchemaFile)
	if err != nil {
		return err
	}
	_, err = db.Exec(string(b))
	if err != nil {
		return err
	}
	slog.Info("schema loaded")
	return nil
}
