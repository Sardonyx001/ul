-- URL shortener database schema

CREATE TABLE IF NOT EXISTS urls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    short_code TEXT NOT NULL UNIQUE,
    original_url TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    clicks INTEGER DEFAULT 0,
    last_clicked_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_short_code ON urls(short_code);
CREATE INDEX IF NOT EXISTS idx_original_url ON urls(original_url);
CREATE INDEX IF NOT EXISTS idx_created_at ON urls(created_at);

-- Click tracking table for detailed analytics
CREATE TABLE IF NOT EXISTS clicks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url_id INTEGER NOT NULL,
    clicked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    user_agent TEXT,
    referer TEXT,
    ip_address TEXT,
    FOREIGN KEY (url_id) REFERENCES urls(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_clicks_url_id ON clicks(url_id);
CREATE INDEX IF NOT EXISTS idx_clicks_clicked_at ON clicks(clicked_at);
