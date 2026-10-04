package service

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	StateDir, Listen, BaseURL, FeedTitle, FeedDescription, FeedLink, ReadToken, Extractor, FFmpegLocation, ExtractorProxy string
	ProcessTimeout, RetryDelay, Retention, PollInterval                                                                   time.Duration
	MaxAttempts                                                                                                           int
	StorageLimitBytes, MinFreeBytes                                                                                       int64
}

func ConfigFromEnv() (Config, error) {
	get := func(key, fallback string) string {
		if value, ok := os.LookupEnv("TWOPOD_" + key); ok {
			return value
		}
		return fallback
	}
	c := Config{StateDir: get("STATE_DIR", "./data"), Listen: get("LISTEN", "127.0.0.1:8080"), FeedTitle: get("FEED_TITLE", "2pod"), FeedDescription: get("FEED_DESCRIPTION", "Selected YouTube audio"), ReadToken: get("READ_TOKEN", ""), Extractor: get("EXTRACTOR", "yt-dlp"), FFmpegLocation: get("FFMPEG_LOCATION", ""), ExtractorProxy: get("EXTRACTOR_PROXY", "")}
	c.BaseURL = strings.TrimRight(get("BASE_URL", "http://"+c.Listen), "/")
	c.FeedLink = get("FEED_LINK", c.BaseURL)
	if c.ExtractorProxy != "" {
		u, err := url.Parse(c.ExtractorProxy)
		valid := err == nil && u.Hostname() != "" && (u.Scheme == "http" || u.Scheme == "https") && (u.Path == "" || u.Path == "/") && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && !strings.HasSuffix(u.Host, ":")
		if valid && u.Port() != "" {
			port, err := strconv.Atoi(u.Port())
			valid = err == nil && port > 0 && port <= 65535
		}
		if !valid {
			return c, fmt.Errorf("TWOPOD_EXTRACTOR_PROXY must be an HTTP(S) proxy URL with a valid host/port and no path, query or fragment")
		}
	}
	var err error
	for _, option := range []struct {
		key, value string
		target     *time.Duration
	}{{"PROCESS_TIMEOUT", "2h", &c.ProcessTimeout}, {"RETRY_DELAY", "1m", &c.RetryDelay}, {"RETENTION", "720h", &c.Retention}, {"POLL_INTERVAL", "1s", &c.PollInterval}} {
		*option.target, err = time.ParseDuration(get(option.key, option.value))
		if err != nil || *option.target <= 0 {
			return c, fmt.Errorf("TWOPOD_%s must be a positive duration", option.key)
		}
	}
	c.MaxAttempts, err = strconv.Atoi(get("MAX_ATTEMPTS", "3"))
	if err != nil || c.MaxAttempts < 1 || c.MaxAttempts > 100 {
		return c, fmt.Errorf("TWOPOD_MAX_ATTEMPTS must be between 1 and 100")
	}
	for _, option := range []struct {
		key, value string
		target     *int64
	}{{"STORAGE_LIMIT_BYTES", "10737418240", &c.StorageLimitBytes}, {"MIN_FREE_BYTES", "536870912", &c.MinFreeBytes}} {
		*option.target, err = strconv.ParseInt(get(option.key, option.value), 10, 64)
		if err != nil || *option.target < 0 || (option.key == "STORAGE_LIMIT_BYTES" && *option.target == 0) {
			return c, fmt.Errorf("TWOPOD_%s has an invalid byte budget", option.key)
		}
	}
	c.StateDir, err = filepath.Abs(c.StateDir)
	if err != nil {
		return c, fmt.Errorf("invalid state directory")
	}
	for _, value := range []string{c.BaseURL, c.FeedLink} {
		u, e := url.Parse(value)
		if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" {
			return c, fmt.Errorf("base URL and feed link must be HTTP(S) URLs without credentials, query or fragment")
		}
	}
	if c.ReadToken != "" && !regexp.MustCompile(`^[A-Za-z0-9_-]{32,}$`).MatchString(c.ReadToken) {
		return c, fmt.Errorf("TWOPOD_READ_TOKEN must contain at least 32 URL-safe characters")
	}
	if c.Extractor == "" || c.Listen == "" {
		return c, fmt.Errorf("extractor and listener must be configured")
	}
	return c, nil
}
