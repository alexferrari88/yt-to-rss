// Package telegram connects the operator's private chat to the durable service.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alexferrari88/yt-to-rss/service"
)

type Config struct {
	Token      string
	OperatorID int64
	APIURL     string
}

func ConfigFromEnv() (Config, error) {
	c := Config{Token: os.Getenv("TWOPOD_TELEGRAM_BOT_TOKEN"), APIURL: os.Getenv("TWOPOD_TELEGRAM_API_URL")}
	id := os.Getenv("TWOPOD_TELEGRAM_OPERATOR_ID")
	if c.Token == "" && id == "" {
		return c, nil
	}
	var err error
	c.OperatorID, err = strconv.ParseInt(id, 10, 64)
	if err != nil || c.OperatorID <= 0 || !regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`).MatchString(c.Token) {
		return Config{}, errors.New("configure both TWOPOD_TELEGRAM_BOT_TOKEN and a positive TWOPOD_TELEGRAM_OPERATOR_ID")
	}
	if c.APIURL == "" {
		c.APIURL = "https://api.telegram.org"
	}
	u, err := url.Parse(c.APIURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1"))) {
		return Config{}, errors.New("TWOPOD_TELEGRAM_API_URL must use HTTPS (HTTP is permitted only on loopback)")
	}
	c.APIURL = strings.TrimRight(c.APIURL, "/")
	return c, nil
}

type update struct {
	ID      int64 `json:"update_id"`
	Message *struct {
		From *struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Chat struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}
type incoming struct {
	ID   int64
	Text string
}
type reply struct {
	Text         string
	SubmissionID string
	State        string
}
type snapshot struct {
	Offset   int64
	Incoming *incoming
	Pending  []reply
	Watching map[string]bool
}
type bot struct {
	cfg    Config
	client *http.Client
	svc    *service.Service
	state  snapshot
	key    string
}

var deliveryUnavailable = errors.New("Telegram reply delivery unavailable")

// Adapter runs outbound polling alongside HTTP/CLI. Provider outages retry with
// a bounded delay; only an inability to preserve local state stops the adapter.
func Adapter(cfg Config) func(context.Context, *service.Service) error {
	return func(ctx context.Context, svc *service.Service) error {
		if cfg.Token == "" {
			<-ctx.Done()
			return nil
		}
		b := &bot{cfg: cfg, svc: svc, client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
		b.key = "telegram:" + strings.SplitN(cfg.Token, ":", 2)[0] + ":" + strconv.FormatInt(cfg.OperatorID, 10)
		raw, err := svc.LoadState(ctx, b.key)
		if err != nil {
			return errors.New("could not load Telegram delivery state")
		}
		if len(raw) > 0 && json.Unmarshal(raw, &b.state) != nil {
			return errors.New("invalid Telegram delivery state")
		}
		if b.state.Watching == nil {
			b.state.Watching = map[string]bool{}
		}
		backoff := time.Second
		deliveryBackoff := time.Second
		var nextDelivery time.Time
		for ctx.Err() == nil {
			if b.state.Incoming != nil {
				if err = b.accept(ctx); err != nil {
					return err
				}
			}
			if err = b.outcomes(ctx); err != nil {
				return err
			}
			if !time.Now().Before(nextDelivery) {
				if err = b.deliver(ctx); err != nil {
					if ctx.Err() != nil {
						return nil
					}
					if !errors.Is(err, deliveryUnavailable) {
						return err
					}
					if nextDelivery.IsZero() {
						log.Print("Telegram reply delivery unavailable; retained for retry")
					}
					nextDelivery = time.Now().Add(deliveryBackoff)
					deliveryBackoff = min(30*time.Second, deliveryBackoff*2)
				} else {
					if !nextDelivery.IsZero() {
						log.Print("Telegram reply delivery recovered")
					}
					nextDelivery = time.Time{}
					deliveryBackoff = time.Second
				}
			}
			var updates []update
			if len(b.state.Pending) < 1000 {
				err = b.call(ctx, "getUpdates", map[string]any{"offset": b.state.Offset, "timeout": 1, "limit": 100, "allowed_updates": []string{"message"}}, &updates)
				if err != nil {
					if ctx.Err() != nil {
						return nil
					}
					log.Print("Telegram polling unavailable; reconnecting")
					if !pause(ctx, backoff) {
						return nil
					}
					backoff = min(30*time.Second, backoff*2)
					continue
				}
				backoff = time.Second
				sort.Slice(updates, func(i, j int) bool { return updates[i].ID < updates[j].ID })
				for _, u := range updates {
					if u.ID < b.state.Offset {
						continue
					}
					m := u.Message
					if m != nil && m.From != nil && m.From.ID == cfg.OperatorID && m.Chat.Type == "private" && m.Chat.ID == cfg.OperatorID {
						// Persist the receipt before Add. A restart resumes this intent,
						// and Add's canonical-source dedup closes the crash window.
						b.state.Incoming = &incoming{ID: u.ID, Text: m.Text}
						if err = b.save(ctx); err != nil {
							return err
						}
						if err = b.accept(ctx); err != nil {
							return err
						}
					} else {
						b.state.Offset = u.ID + 1
						if err = b.save(ctx); err != nil {
							return err
						}
					}
				}
			}
			if !pause(ctx, 200*time.Millisecond) {
				return nil
			}
		}
		return nil
	}
}

func (b *bot) save(ctx context.Context) error {
	data, err := json.Marshal(b.state)
	if err == nil {
		err = b.svc.SaveState(ctx, b.key, data)
	}
	if err != nil {
		return errors.New("could not persist Telegram delivery state")
	}
	return nil
}
func (b *bot) accept(ctx context.Context) error {
	text := strings.TrimSpace(b.state.Incoming.Text)
	r := reply{Text: "Send one public or unlisted YouTube video link. Use /status VIDEO_ID to inspect it."}
	switch {
	case text == "/start" || text == "/help":
	case strings.HasPrefix(text, "/status "):
		s, err := b.svc.Status(ctx, strings.TrimSpace(strings.TrimPrefix(text, "/status ")))
		if err != nil {
			r.Text = "Submission not found. Use the video ID returned when you shared it."
		} else {
			r.Text = "Status: " + s.ID + " — " + s.State
		}
	default:
		var links []string
		for _, word := range strings.Fields(text) {
			if strings.HasPrefix(word, "https://") || strings.HasPrefix(word, "http://") {
				links = append(links, word)
			}
		}
		if len(links) == 1 {
			s, err := b.svc.Add(ctx, links[0])
			if err != nil {
				r.Text = "Not accepted. Send a valid individual YouTube video link; playlists and other sites are unsupported. Check the CLI if it still fails."
			} else {
				r.SubmissionID = s.ID
				r.State = s.State
				r.Text = strings.ToUpper(s.State[:1]) + s.State[1:] + ": " + s.ID
				if s.State == "queued" || s.State == "processing" {
					b.state.Watching[s.ID] = true
				}
				if s.State == "failed" {
					r.Text = b.failed(s)
				}
			}
		}
	}
	b.state.Pending = append(b.state.Pending, r)
	b.state.Offset = b.state.Incoming.ID + 1
	b.state.Incoming = nil
	return b.save(ctx)
}
func (b *bot) outcomes(ctx context.Context) error {
	changed := false
	for id := range b.state.Watching {
		s, err := b.svc.Status(ctx, id)
		if err != nil {
			return errors.New("could not inspect Telegram submission")
		}
		r := reply{SubmissionID: id, State: s.State}
		switch s.State {
		case "published":
			r.Text = "Published: " + id + ". Available in your podcast feed."
		case "failed":
			r.Text = b.failed(s)
		case "deleted", "expired":
			r.Text = "Status: " + id + " — " + s.State
		default:
			continue
		}
		b.state.Pending = append(b.state.Pending, r)
		delete(b.state.Watching, id)
		changed = true
	}
	if changed {
		return b.save(ctx)
	}
	return nil
}
func (b *bot) failed(s service.Submission) string {
	// The service's failures are sanitized, fixed diagnostics. Keep credentials
	// and URLs out even if a future extractor adds richer failure descriptions.
	reason := strings.ReplaceAll(s.Failure, b.cfg.Token, "[redacted]")
	reason = regexp.MustCompile(`https?://\S+`).ReplaceAllString(reason, "[URL omitted]")
	if len(reason) > 500 {
		reason = reason[:500]
	}
	return "Failed: " + s.ID + ". " + reason + " Inspect with CLI status; fix the cause and use CLI retry."
}
func (b *bot) deliver(ctx context.Context) error {
	for len(b.state.Pending) > 0 {
		r := b.state.Pending[0]
		if r.SubmissionID != "" && r.State == "published" {
			s, err := b.svc.Status(ctx, r.SubmissionID)
			if err != nil {
				return errors.New("could not inspect pending publication reply")
			}
			if s.State != "published" {
				r.Text = "Status: " + s.ID + " — " + s.State
			}
		}
		if err := b.call(ctx, "sendMessage", map[string]any{"chat_id": b.cfg.OperatorID, "text": r.Text, "link_preview_options": map[string]bool{"is_disabled": true}}, nil); err != nil {
			return deliveryUnavailable
		}
		// Telegram has no idempotency key. A lost response or a crash before this
		// save may repeat a reply; preserving undelivered outcomes takes priority.
		b.state.Pending = b.state.Pending[1:]
		if err := b.save(ctx); err != nil {
			return err
		}
	}
	return nil
}
func (b *bot) call(ctx context.Context, method string, payload any, result any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return errors.New("invalid Telegram request")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", b.cfg.APIURL+"/bot"+b.cfg.Token+"/"+method, bytes.NewReader(data))
	if err != nil {
		return errors.New("invalid Telegram endpoint")
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := b.client.Do(req)
	if err != nil {
		return errors.New("Telegram network request failed")
	}
	defer response.Body.Close()
	var envelope struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
	}
	if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&envelope) != nil || !envelope.OK {
		return errors.New("Telegram request rejected or unavailable")
	}
	if result != nil && json.Unmarshal(envelope.Result, result) != nil {
		return fmt.Errorf("invalid Telegram response")
	}
	return nil
}
func pause(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
