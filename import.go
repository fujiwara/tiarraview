package tiarraview

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func runImport(ctx context.Context) error {
	db, err := openDB()
	if err != nil {
		return fmt.Errorf("failed to open DB: %w", err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "DELETE FROM tiarra"); err != nil {
		return fmt.Errorf("failed to truncate table: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	id := 0
	if err := filepath.WalkDir(config.Import.SrcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk dir: %w", err)
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".txt" {
			slog.Info("skip", "file", path)
			return nil
		}
		slog.Info("import", "file", path)
		id++
		if err := importLog(ctx, tx, path, id); err != nil {
			return fmt.Errorf("failed to import log: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func importLog(ctx context.Context, db *sql.Tx, path string, id int) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	ps := strings.Split(path, "/")
	channel := ps[len(ps)-2]
	filename := ps[len(ps)-1]
	logDate := strings.TrimSuffix(filename, ".txt")

	lines := strings.Split(string(b), "\n")
	contents := make([]string, 0, len(lines))
	ngrams := make([]string, 0, len(lines))
	for _, line := range lines {
		content, ng := parseLogLine(line)
		if content == "" {
			continue
		}
		contents = append(contents, line)
		ngrams = append(ngrams, ng...)
	}
	if len(contents) == 0 {
		slog.Info("no contents", "file", path)
		return nil
	}

	st1, err := db.PrepareContext(ctx, "INSERT INTO tiarra (id, channel, log_date, content) VALUES (?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer st1.Close()
	content := strings.Join(contents, "\n")
	if _, err := st1.ExecContext(ctx, id, channel, logDate, content); err != nil {
		return fmt.Errorf("failed to insert log: %w", err)
	}

	if len(ngrams) > 0 {
		st2, err := db.PrepareContext(ctx, "INSERT INTO tiarra_fts (id, content) VALUES (?, ?)")
		if err != nil {
			return fmt.Errorf("failed to prepare statement fts: %w", err)
		}
		defer st2.Close()
		ngs := strings.Join(ngrams, " ")
		if _, err := st2.ExecContext(ctx, id, ngs); err != nil {
			return fmt.Errorf("failed to insert fts: %w", err)
		}
	}

	slog.Info("imported", "id", id, "file", path, "bytes", len(b), "lines", len(contents))
	return nil
}

func parseLogLine(line string) (string, []string) {
	p := strings.SplitN(line, " ", 2)
	if len(p) != 2 {
		return "", nil
	}
	if strings.HasPrefix(p[1], "<") || strings.HasPrefix(p[1], "(") {
		bs := strings.SplitN(p[1], " ", 2)
		if len(bs) != 2 {
			return "", nil
		}
		return line, createNgrams(bs[1], 2)
	}
	return "", nil
}

func createNgrams(input string, n int) []string {
	runes := []rune(strings.ToLower(input))
	var ngrams []string
	for i := 0; i < len(runes)-n+1; i++ {
		ng := string(runes[i : i+n])
		ngrams = append(ngrams, ng)
	}
	return ngrams
}
