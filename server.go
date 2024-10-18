package tiarraview

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/fujiwara/ridge"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
)

func runServer(ctx context.Context) error {
	e := echo.New()
	e.Renderer = newTemplates()
	e.GET("/", rootHandler)
	e.GET("/static/*", staticHandler)
	e.GET("/:channel/", channelHandler)
	e.GET("/:channel/:log_date", contentsHandler)
	e.GET("/search", searchHandler)
	ridge.RunWithContext(ctx, config.Server.Addr, "/", e)
	return nil
}

func errorResponse(c echo.Context, status int, err error) error {
	slog.Error(err.Error())
	return c.String(status, http.StatusText(status))
}

func rootHandler(c echo.Context) error {
	ctx := c.Request().Context()
	db, err := openDB()
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to open DB: %w", err))
	}
	defer db.Close()
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to query DB: %w", err))
	}
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
	db, err := openDB()
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
	ID      int    `db:"id"`
	Channel string `db:"channel"`
	LogDate string `db:"log_date"`
	Content string `db:"content"`
}

func contentsHandler(c echo.Context) error {
	ctx := c.Request().Context()
	channel, _ := url.PathUnescape(c.Param("channel"))
	logDate := c.Param("log_date")

	db, err := openDB()
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to open DB: %w", err))
	}
	defer db.Close()
	logs := []TiarraLog{}
	if err := db.SelectContext(ctx, &logs, "SELECT content FROM tiarra WHERE channel = ? AND log_date = ? ORDER BY id LIMIT 1", channel, logDate); err != nil {
		return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to query DB: %w", err))
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

func searchHandler(c echo.Context) error {
	ctx := c.Request().Context()
	db, err := openDB()
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to open DB: %w", err))
	}
	defer db.Close()
	logs := []TiarraLog{}
	matches := []string{}
	q := c.QueryParam("search")
	if len(q) < 2 {
		q = q + " "
	}
	firstWord := strings.ToLower(strings.Split(q, " ")[0])
	channel := url.PathEscape(c.QueryParam("channel"))
	ng := createNgrams(q, 2)
	for _, n := range ng {
		if strings.Contains(n, `"`) {
			continue
		}
		matches = append(matches, `"`+n+`"`)
	}
	if channel == "" {
		if err := db.SelectContext(ctx, &logs,
			`SELECT DISTINCT tiarra.* FROM tiarra JOIN tiarra_fts USING(id)
			WHERE tiarra_fts MATCH ? ORDER BY rank LIMIT 100`,
			strings.Join(matches, " "),
		); err != nil {
			return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to query DB: %w", err))
		}
	} else {
		if err := db.SelectContext(ctx, &logs,
			`SELECT DISTINCT tiarra.* FROM tiarra JOIN tiarra_fts USING(id)
			WHERE tiarra_fts MATCH ? AND channel = ? ORDER BY rank LIMIT 100`,
			strings.Join(matches, " "), channel,
		); err != nil {
			return errorResponse(c, http.StatusInternalServerError, fmt.Errorf("failed to query DB: %w", err))
		}
	}
	filteredLogs := []TiarraLog{}
	channels := []string{}
	for _, log := range logs {
		var content string
		for _, line := range strings.Split(log.Content, "\n") {
			if strings.Contains(strings.ToLower(line), firstWord) {
				content += line + "\n"
			}
		}
		if content != "" {
			log.Content = content
			filteredLogs = append(filteredLogs, log)
			channels = append(channels, log.Channel)
		}
	}
	channels = lo.Uniq(channels)
	sort.Strings(channels)

	return c.Render(http.StatusOK, "search.html", map[string]interface{}{
		"Title":    fmt.Sprintf("IRC Log Viewer / Search: %s", c.QueryParam("search")),
		"Logs":     filteredLogs,
		"Query":    c.QueryParam("search"),
		"Channels": channels,
	})
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

	// ファイルの内容を返す
	return c.Stream(http.StatusOK, "text/css", file)
}
