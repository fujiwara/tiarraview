CREATE TABLE tiarra (
    channel VARCHAR(255) NOT NULL,
    log_date VARCHAR(255) NOT NULL,
    content TEXT NOT NULL
);
CREATE UNIQUE INDEX tiarra_channel_log_date ON tiarra (channel, log_date);
CREATE VIRTUAL TABLE tiarra_fts using fts5(content, tokenize="trigram", content='');
