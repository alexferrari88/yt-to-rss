package acceptance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type telegramFixture struct {
	mu           sync.Mutex
	server       *httptest.Server
	updates      []map[string]any
	replies      []string
	offset       int64
	blockReplies bool
	replayAll    bool
	blockPolling bool
}

func newTelegramFixture(t *testing.T) *telegramFixture {
	f := &telegramFixture{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		var payload struct {
			Offset int64  `json:"offset"`
			ChatID int64  `json:"chat_id"`
			Text   string `json:"text"`
		}
		if json.NewDecoder(r.Body).Decode(&payload) != nil {
			http.Error(w, "bad payload", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			if f.blockPolling {
				http.Error(w, "private provider failure must not be logged", 503)
				return
			}
			f.offset = payload.Offset
			updates := []map[string]any{}
			for _, u := range f.updates {
				if f.replayAll || u["update_id"].(int64) >= payload.Offset {
					updates = append(updates, u)
				}
			}
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": updates})
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			if payload.ChatID != 4242 {
				t.Errorf("reply delivered outside operator private chat: %d", payload.ChatID)
			}
			if f.blockReplies {
				w.WriteHeader(503)
				json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "provider response containing credentials must not be logged"})
				return
			}
			f.replies = append(f.replies, payload.Text)
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"message_id": len(f.replies)}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}
func (f *telegramFixture) settings() []string {
	return []string{"TWOPOD_TELEGRAM_BOT_TOKEN=123456:fixture_secret_token", "TWOPOD_TELEGRAM_OPERATOR_ID=4242", "TWOPOD_TELEGRAM_API_URL=" + f.server.URL}
}
func (f *telegramFixture) share(id, sender, chat int64, kind, text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, map[string]any{"update_id": id, "message": map[string]any{"message_id": id, "from": map[string]any{"id": sender}, "chat": map[string]any{"id": chat, "type": kind}, "text": text}})
}
func (f *telegramFixture) waitReply(t *testing.T, text string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		for _, r := range f.replies {
			if strings.Contains(r, text) {
				f.mu.Unlock()
				return
			}
		}
		f.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	t.Fatalf("no Telegram reply containing %q: %v", text, f.replies)
}
func (f *telegramFixture) waitOffset(t *testing.T, offset int64) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		ok := f.offset >= offset
		f.mu.Unlock()
		if ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("Telegram polling cursor did not advance")
}

func TestTelegramAuthorizedSubmissionUsesCLILifecycle(t *testing.T) {
	f := newTelegramFixture(t)
	h := newHarness(t, f.settings()...)
	f.share(1, 9999, 9999, "private", "https://youtu.be/abcdefghijk")
	f.share(2, 4242, -55, "group", "https://youtu.be/abcdefghijk")
	f.share(3, 4242, 9999, "private", "https://youtu.be/abcdefghijk")
	f.waitOffset(t, 4)
	if got := strings.TrimSpace(string(h.run("list"))); got != "[]" {
		t.Fatalf("unauthorized messages created work: %s", got)
	}
	if err := os.WriteFile(filepath.Join(h.dir, "mode-abcdefghijk"), []byte("wait"), 0600); err != nil {
		t.Fatal(err)
	}
	f.share(4, 4242, 4242, "private", "https://youtu.be/abcdefghijk?t=99")
	f.waitReply(t, "Queued: abcdefghijk")
	h.wait("abcdefghijk", "processing")
	if s := h.add("https://www.youtube.com/watch?v=abcdefghijk"); s.ID != "abcdefghijk" || s.State != "processing" {
		t.Fatalf("CLI/bot active source differs: %+v", s)
	}
	os.Remove(filepath.Join(h.dir, "mode-abcdefghijk"))
	h.wait("abcdefghijk", "published")
	f.waitReply(t, "Published: abcdefghijk")
	rss, _ := h.feed()
	if len(rss.Channel.Items) != 1 {
		t.Fatal("Telegram submission was not published once")
	}
}

func TestTelegramReplayAndCLISharingDoNotDuplicatePublication(t *testing.T) {
	f := newTelegramFixture(t)
	h := newHarness(t, f.settings()...)
	s := h.add("https://youtu.be/abcdefghijk")
	h.wait(s.ID, "published")
	f.share(10, 4242, 4242, "private", "https://m.youtube.com/watch?v=abcdefghijk&t=40")
	f.waitReply(t, "Published: abcdefghijk")
	f.waitOffset(t, 11)
	h.stop()
	f.mu.Lock()
	before := len(f.replies)
	f.replayAll = true
	f.mu.Unlock()
	h.start()
	// The provider deliberately returns an already acknowledged update. The
	// application must also guard replay using its persisted cursor.
	f.share(10, 4242, 4242, "private", "https://youtube.com/shorts/abcdefghijk")
	f.share(11, 4242, 4242, "private", "/status abcdefghijk")
	f.waitReply(t, "Status: abcdefghijk — published")
	f.waitOffset(t, 12)
	f.mu.Lock()
	after := len(f.replies)
	f.mu.Unlock()
	if after != before+1 {
		t.Fatalf("replayed update generated another reply: before %d after %d", before, after)
	}
	rss, _ := h.feed()
	if len(rss.Channel.Items) != 1 || h.status(s.ID).Attempts != 1 {
		t.Fatal("cross-CLI sharing or replay extracted/published duplicate")
	}
}

func TestTelegramTerminalRepliesSurviveDeliveryOutageAndRestart(t *testing.T) {
	f := newTelegramFixture(t)
	h := newHarness(t, append(f.settings(), "TWOPOD_MAX_ATTEMPTS=1")...)
	f.mu.Lock()
	f.blockReplies = true
	f.mu.Unlock()
	os.WriteFile(filepath.Join(h.dir, "mode-failsssssss"), []byte("fail"), 0600)
	f.share(1, 4242, 4242, "private", "https://youtu.be/abcdefghijk")
	f.share(2, 4242, 4242, "private", "https://youtu.be/failsssssss")
	f.share(3, 4242, 4242, "private", "https://example.com/unsupported")
	f.waitOffset(t, 4)
	h.wait("abcdefghijk", "published")
	h.wait("failsssssss", "failed")
	// No provider delivery succeeds, while CLI status and HTTP remain useful.
	rss, _ := h.feed()
	if len(rss.Channel.Items) != 1 {
		t.Fatal("Telegram outage affected feed")
	}
	h.stop()
	f.mu.Lock()
	replyCount := len(f.replies)
	f.blockReplies = false
	f.mu.Unlock()
	if replyCount != 0 {
		t.Fatal("rejected delivery reported successful")
	}
	h.start()
	f.waitReply(t, "Published: abcdefghijk")
	f.waitReply(t, "Failed: failsssssss")
	f.waitReply(t, "Not accepted.")
	f.mu.Lock()
	replies := strings.Join(f.replies, "\n")
	f.mu.Unlock()
	if !strings.Contains(replies, "CLI retry") || strings.Contains(replies, "fixture_secret_token") || strings.Contains(replies, strings.Repeat("s", 40)) || strings.Contains(replies, "secret-provider-output") {
		t.Fatalf("failure reply lacks action or exposes credentials: %s", replies)
	}
	h.stop()
	if logs := h.log.String(); strings.Contains(logs, "fixture_secret_token") || strings.Contains(logs, "provider response containing credentials") {
		t.Fatalf("unsafe Telegram diagnostic: %s", logs)
	}
}

func TestTelegramPollingReconnectsWithoutBlockingCLIOrHTTP(t *testing.T) {
	f := newTelegramFixture(t)
	f.mu.Lock()
	f.blockPolling = true
	f.mu.Unlock()
	h := newHarness(t, f.settings()...)
	s := h.add("https://youtu.be/abcdefghijk")
	h.wait(s.ID, "published")
	rss, _ := h.feed()
	if len(rss.Channel.Items) != 1 {
		t.Fatal("polling outage blocked CLI publication or HTTP")
	}
	f.share(1, 4242, 4242, "private", "https://youtu.be/abcdefghijk")
	f.mu.Lock()
	f.blockPolling = false
	f.mu.Unlock()
	f.waitReply(t, "Published: abcdefghijk")
	f.waitOffset(t, 2)
	if h.status(s.ID).Attempts != 1 {
		t.Fatal("reconnection duplicated CLI extraction")
	}
}
