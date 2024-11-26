package tiarraview

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/fujiwara/ridge"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
	sloghttp "github.com/samber/slog-http"
)

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

func runServer(ctx context.Context) error {
	e := echo.New()
	e.Renderer = newTemplates()
	e.GET("/", rootHandler)
	e.GET("/static/*", staticHandler)
	e.GET("/log/:channel/", channelHandler)
	e.GET("/log/:channel/:log_date", contentsHandler)
	e.GET("/search", searchHandler)

	// add logger middleware
	handler := sloghttp.Recovery(e)
	handler = sloghttp.New(logger)(handler)
	ridge.RunWithContext(ctx, config.Server.Addr, config.Server.Root, handler)
	return nil
}

func errorResponse(c echo.Context, status int, err error) error {
	logger.Error(err.Error())
	return c.String(status, http.StatusText(status))
}

func rootHandler(c echo.Context) error {
	ctx := c.Request().Context()
	db, err := openDB(ctx)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to open DB: %w", err))
	}
	defer db.Close()

	channels, err := listChannels(ctx, db)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to query DB: %w", err))
	}
	return c.Render(http.StatusOK, "index.html", map[string]interface{}{
		"Title":    "IRC Log Viewer",
		"Channels": channels,
		"Query":    "",
	})
}

func listChannels(ctx context.Context, db *sqlx.DB) ([]string, error) {
	var channels []string
	rows, err := db.QueryContext(ctx, "SELECT distinct channel FROM tiarra ORDER BY channel")
	if err != nil {
		return nil, fmt.Errorf("failed to query DB: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var channel string
		if err := rows.Scan(&channel); err != nil {
			return nil, fmt.Errorf("failed to scan rows: %w", err)
		}
		channels = append(channels, channel)
	}
	return channels, nil
}

func channelHandler(c echo.Context) error {
	ctx := c.Request().Context()
	channel, _ := url.PathUnescape(c.Param("channel"))
	db, err := openDB(ctx)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to open DB: %w", err))
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, "SELECT distinct log_date FROM tiarra WHERE channel = ? ORDER BY log_date DESC", channel)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to query DB: %w", err))
	}
	defer rows.Close()
	var logDates []string
	for rows.Next() {
		var logDate string
		if err := rows.Scan(&logDate); err != nil {
			return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to scan rows: %w", err))
		}
		logDates = append(logDates, logDate)
	}
	return c.Render(http.StatusOK, "channel.html", map[string]interface{}{
		"Title":    fmt.Sprintf("IRC Log Viewer / %s", channel),
		"Channel":  channel,
		"Channels": []string{channel},
		"LogDates": logDates,
		"Query":    "",
	})
}

type TiarraLog struct {
	ID      int    `db:"rowid"`
	Channel string `db:"channel"`
	LogDate string `db:"log_date"`
	Content string `db:"content"`
}

func contentsHandler(c echo.Context) error {
	ctx := c.Request().Context()
	channel, err := url.PathUnescape(c.Param("channel"))
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, fmt.Errorf("failed to unescape channel: %w", err))
	}
	logDate := strings.TrimSuffix(c.Param("log_date"), ".txt")

	db, err := openDB(ctx)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to open DB: %w", err))
	}
	defer db.Close()
	logs := []TiarraLog{}
	if err := db.SelectContext(ctx, &logs, "SELECT content FROM tiarra WHERE channel = ? AND log_date = ?", channel, logDate); err != nil {
		return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to query DB: %w", err))
	}
	if len(logs) == 0 {
		return c.String(http.StatusNotFound, "Not Found")
	}

	return c.Render(http.StatusOK, "contents.html", map[string]interface{}{
		"Title":    fmt.Sprintf("IRC Log Viewer / %s / %s", channel, logDate),
		"Channel":  channel,
		"Channels": []string{channel},
		"LogDate":  logDate,
		"Content":  logs[0].Content,
		"Query":    "",
	})
}

func quoteMatch(s string) string {
	ws := []string{}
	for _, w := range strings.Split(s, " ") {
		w = strings.ReplaceAll(w, `"`, ``)
		w = strings.Trim(w, ` `)
		ws = append(ws, `"`+w+`"`)
	}
	return strings.Join(ws, " ")
}

func searchHandler(c echo.Context) error {
	ctx := c.Request().Context()
	db, err := openDB(ctx)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to open DB: %w", err))
	}
	defer db.Close()
	logs := []TiarraLog{}
	q := c.QueryParam("search")
	firstWord := strings.ToLower(strings.Split(q, " ")[0])
	channel, err := url.PathUnescape(c.QueryParam("channel"))
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, fmt.Errorf("failed to unescape channel: %w", err))
	}
	match := quoteMatch(q)
	if channel == "" {
		query := `SELECT DISTINCT tiarra.* FROM tiarra JOIN tiarra_fts ON (tiarra.rowid=tiarra_fts.rowid)
				WHERE tiarra_fts MATCH ? ORDER BY rank LIMIT 100`
		logger.Info("searching", "query", query, "match", match)
		if err := db.SelectContext(ctx, &logs, query, match); err != nil {
			return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to query DB: %w", err))
		}
	} else {
		logger.Info("searching", "channel", channel, "match", match)
		if err := db.SelectContext(ctx, &logs,
			`SELECT DISTINCT tiarra.* FROM tiarra JOIN tiarra_fts ON (tiarra.rowid=tiarra_fts.rowid)
			WHERE tiarra_fts MATCH ? AND channel = ? ORDER BY rank LIMIT 100`,
			match, channel,
		); err != nil {
			return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to query DB: %w", err))
		}
	}
	logger.Info("search result", "logs", len(logs))
	filteredLogs := []TiarraLog{}
	channels := []string{}
	for _, log := range logs {
		var content string
		matched := 0
		for _, line := range strings.Split(log.Content, "\n") {
			msg, _ := parseLogLine(line)
			if strings.Contains(strings.ToLower(msg), firstWord) {
				content += line + "\n"
				logger.Debug("matched", "msg", msg)
				matched++
			}
			if matched > 10 {
				break
			}
		}
		if content != "" {
			log.Content = content
		} else {
			log.Content = prefix(log.Content, 256)
		}
		filteredLogs = append(filteredLogs, log)
		channels = append(channels, log.Channel)
	}
	channels = lo.Uniq(channels)
	sort.Strings(channels)

	v := map[string]interface{}{
		"Title":    fmt.Sprintf("IRC Log Viewer / Search: %s", c.QueryParam("search")),
		"Logs":     filteredLogs,
		"Query":    c.QueryParam("search"),
		"Channels": channels,
		"Channel":  channel,
	}
	if utf8.RuneCountInString(q) <= 2 {
		v["Warning"] = true
	}
	return c.Render(http.StatusOK, "search.html", v)
}

func prefix(s string, n int) string {
	if utf8.RuneCountInString(s) > n {
		runes := []rune(s)
		return string(runes[:n])
	} else {
		return s
	}
}

//go:embed static/*
var staticFiles embed.FS

func staticHandler(c echo.Context) error {
	filePath := c.Param("*")
	file, err := staticFiles.Open("static/" + filePath)
	if err != nil {
		return c.String(http.StatusNotFound, "File not found")
	}
	defer file.Close()

	c.Response().Header().Set("cache-control", "public, max-age=86400")
	return c.Stream(http.StatusOK, "text/css", file)
}
