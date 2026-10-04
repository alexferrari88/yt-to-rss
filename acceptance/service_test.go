package acceptance

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

var binary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "2pod-tests-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	binary = filepath.Join(dir, "2pod")
	cmd := exec.Command("go", "build", "-o", binary, "../cmd/2pod")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build CLI: %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type submission struct {
	ID, SourceURL, State, Title, Uploader, Failure, PausedReason string
	Attempts                                                     int
	Duration                                                     float64
	PublishedAt                                                  *time.Time
}
type rssDocument struct {
	Version string `xml:"version,attr"`
	Channel struct {
		Title       string `xml:"title"`
		Description string `xml:"description"`
		Link        string `xml:"link"`
		Items       []struct {
			Title       string `xml:"title"`
			Description string `xml:"description"`
			Link        string `xml:"link"`
			PubDate     string `xml:"pubDate"`
			GUID        struct {
				Value       string `xml:",chardata"`
				IsPermaLink string `xml:"isPermaLink,attr"`
			} `xml:"guid"`
			Enclosure struct {
				URL    string `xml:"url,attr"`
				Type   string `xml:"type,attr"`
				Length int64  `xml:"length,attr"`
			} `xml:"enclosure"`
		} `xml:"item"`
	} `xml:"channel"`
}
type harness struct {
	t         *testing.T
	dir, base string
	env       []string
	daemon    *exec.Cmd
	log       bytes.Buffer
}

func newHarness(t *testing.T, settings ...string) *harness {
	t.Helper()
	dir := t.TempDir()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	fixture, err := filepath.Abs("testdata/tone.mp3")
	if err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("testdata/extractor.py")
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, dir: dir, base: "http://" + addr}
	for _, setting := range os.Environ() {
		if !strings.HasPrefix(setting, "TWOPOD_") {
			h.env = append(h.env, setting)
		}
	}
	h.env = append(h.env, "TWOPOD_STATE_DIR="+filepath.Join(dir, "state"), "TWOPOD_LISTEN="+addr, "TWOPOD_BASE_URL="+h.base, "TWOPOD_READ_TOKEN="+strings.Repeat("s", 40), "TWOPOD_EXTRACTOR="+script, "TWOPOD_POLL_INTERVAL=20ms", "TWOPOD_RETRY_DELAY=40ms", "TWOPOD_MIN_FREE_BYTES=0", "FIXTURE_MP3="+fixture, "FIXTURE_CONTROL="+dir)
	h.env = append(h.env, settings...)
	h.start()
	t.Cleanup(h.stop)
	return h
}
func (h *harness) start() {
	h.t.Helper()
	h.daemon = exec.Command(binary, "serve")
	h.daemon.Env = h.env
	h.daemon.Stdout = &h.log
	h.daemon.Stderr = &h.log
	if err := h.daemon.Start(); err != nil {
		h.t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(h.dir, "state/control.sock")); err == nil {
			if _, err := h.command("list"); err == nil {
				return
			}
		}
		time.Sleep(15 * time.Millisecond)
	}
	h.t.Fatalf("daemon startup: %s", h.log.String())
}
func (h *harness) stop() {
	if h.daemon == nil {
		return
	}
	h.daemon.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- h.daemon.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		h.daemon.Process.Kill()
		<-done
	}
	h.daemon = nil
}
func (h *harness) command(args ...string) ([]byte, error) {
	cmd := exec.Command(binary, args...)
	cmd.Env = h.env
	return cmd.CombinedOutput()
}
func (h *harness) run(args ...string) []byte {
	h.t.Helper()
	out, err := h.command(args...)
	if err != nil {
		h.t.Fatalf("CLI %v: %v %s", args, err, out)
	}
	return out
}
func (h *harness) add(url string) submission {
	h.t.Helper()
	var s submission
	if err := json.Unmarshal(h.run("add", url), &s); err != nil {
		h.t.Fatal(err)
	}
	return s
}
func (h *harness) status(id string) submission {
	h.t.Helper()
	var s submission
	if err := json.Unmarshal(h.run("status", id), &s); err != nil {
		h.t.Fatal(err)
	}
	return s
}
func (h *harness) wait(id, state string) submission {
	h.t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		s := h.status(id)
		if s.State == state {
			return s
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatalf("%s never reached %s: %+v logs=%s", id, state, h.status(id), h.log.String())
	return submission{}
}
func (h *harness) feed() (rssDocument, []byte) {
	h.t.Helper()
	r, err := http.Get(h.base + "/" + strings.Repeat("s", 40) + "/feed.xml")
	if err != nil {
		h.t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	if r.StatusCode != 200 {
		h.t.Fatalf("feed status%d %s", r.StatusCode, b)
	}
	var rss rssDocument
	if err := xml.Unmarshal(b, &rss); err != nil {
		h.t.Fatal(err)
	}
	return rss, b
}

func TestCLIToPlayableFeed(t *testing.T) {
	h := newHarness(t)
	start := time.Now().Add(-time.Second)
	s := h.add("https://youtu.be/abcdefghijk?t=81&list=ignored")
	if s.ID != "abcdefghijk" || s.SourceURL != "https://www.youtube.com/watch?v=abcdefghijk" {
		t.Fatalf("canonical submission: %+v", s)
	}
	s = h.wait(s.ID, "published")
	if s.Title != "An <old> video & audio" || s.Uploader != "Fixture Creator" || s.Duration != 1 {
		t.Fatalf("metadata %+v", s)
	}
	duplicate := h.add("https://www.youtube.com/watch?v=abcdefghijk&start=25")
	if duplicate.ID != s.ID || duplicate.State != "published" {
		t.Fatalf("dedup %+v", duplicate)
	}
	rss, _ := h.feed()
	if rss.Version != "2.0" || len(rss.Channel.Items) != 1 {
		t.Fatalf("RSS %+v", rss)
	}
	item := rss.Channel.Items[0]
	if item.Title != s.Title || item.GUID.Value != "youtube:abcdefghijk" || item.GUID.IsPermaLink != "false" || item.Link != s.SourceURL {
		t.Fatalf("RSS item %+v", item)
	}
	date, err := time.Parse(time.RFC1123Z, item.PubDate)
	if err != nil || date.Before(start) {
		t.Fatalf("publication date %q %v", item.PubDate, err)
	}
	fixture, err := os.ReadFile("testdata/tone.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if item.Enclosure.Type != "audio/mpeg" || item.Enclosure.Length != int64(len(fixture)) || item.Enclosure.URL != h.base+"/"+strings.Repeat("s", 40)+"/media/abcdefghijk.mp3" {
		t.Fatalf("enclosure %+v", item.Enclosure)
	}
	r, err := http.Get(item.Enclosure.URL)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != 200 || !bytes.Equal(b, fixture) || r.Header.Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("MP3 status%d bytes%d", r.StatusCode, len(b))
	}
	req, _ := http.NewRequest("HEAD", item.Enclosure.URL, nil)
	r, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 200 || r.ContentLength != int64(len(fixture)) {
		t.Fatalf("HEAD %+v", r)
	}
	req, _ = http.NewRequest("GET", item.Enclosure.URL, nil)
	req.Header.Set("Range", "bytes=10-19")
	r, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != 206 || !bytes.Equal(b, fixture[10:20]) || r.Header.Get("Content-Range") != "bytes 10-19/"+strconv.Itoa(len(fixture)) {
		t.Fatalf("range %d %q %v", r.StatusCode, b, r.Header)
	}
	for _, path := range []string{"/feed.xml", "/wrong/feed.xml", "/wrong/media/abcdefghijk.mp3", "/" + strings.Repeat("s", 40) + "/media/missingxxxx.mp3", "/" + strings.Repeat("s", 40) + "/add"} {
		r, err = http.Get(h.base + path)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != 404 {
			t.Fatalf("unauthorized/missing path %s status%d", path, r.StatusCode)
		}
	}
	h.stop()
	h.start()
	rss, _ = h.feed()
	if len(rss.Channel.Items) != 1 || rss.Channel.Items[0].GUID.Value != "youtube:abcdefghijk" || h.status(s.ID).State != "published" {
		t.Fatal("restart lost publication")
	}
	info, err := os.Stat(filepath.Join(h.dir, "state/control.sock"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("management socket permissions %v %v", info, err)
	}
}

func TestInvalidSourcesAndIncompleteWorkNeverPublish(t *testing.T) {
	h := newHarness(t, "TWOPOD_MAX_ATTEMPTS=1")
	for _, u := range []string{"https://example.com/watch?v=abcdefghijk", "https://youtube.com/playlist?list=abc", "https://youtube.com/watch?v=bad", "https://youtube.com.evil.example/watch?v=abcdefghijk", "file:///etc/passwd", "https://user:pass@youtube.com/watch?v=abcdefghijk"} {
		if out, err := h.command("add", u); err == nil {
			t.Fatalf("accepted invalid source %s: %s", u, out)
		}
	}
	if err := os.WriteFile(filepath.Join(h.dir, "mode-abcdefghijk"), []byte("wait"), 0600); err != nil {
		t.Fatal(err)
	}
	first := h.add("https://youtube.com/shorts/abcdefghijk")
	h.wait(first.ID, "processing")
	duplicate := h.add("https://m.youtube.com/watch?v=abcdefghijk&t=12")
	if duplicate.ID != first.ID {
		t.Fatal("queued dedup failed")
	}
	other := h.add("https://youtube.com/embed/lmnopqrstuv")
	time.Sleep(100 * time.Millisecond)
	if h.status(other.ID).State != "queued" {
		t.Fatal("more than one extraction ran")
	}
	rss, _ := h.feed()
	if len(rss.Channel.Items) != 0 {
		t.Fatal("partial work published")
	}
	os.Remove(filepath.Join(h.dir, "mode-abcdefghijk"))
	h.wait(first.ID, "published")
	h.wait(other.ID, "published")
	os.WriteFile(filepath.Join(h.dir, "mode-failsssssss"), []byte("fail"), 0600)
	failed := h.add("https://youtu.be/failsssssss")
	failed = h.wait(failed.ID, "failed")
	if failed.Failure == "" || strings.Contains(failed.Failure, "secret") || strings.Contains(failed.Failure, strings.Repeat("s", 40)) {
		t.Fatalf("failure not sanitized: %+v", failed)
	}
	os.WriteFile(filepath.Join(h.dir, "mode-livesssssss"), []byte("live"), 0600)
	live := h.add("https://youtu.be/livesssssss")
	live = h.wait(live.ID, "failed")
	if !strings.Contains(live.Failure, "Unsupported") {
		t.Fatalf("live outcome %+v", live)
	}
	rss, _ = h.feed()
	if len(rss.Channel.Items) != 2 {
		t.Fatalf("incomplete extraction entered feed: %+v", rss)
	}
}

func TestBoundedRetriesAndManualRecovery(t *testing.T) {
	h := newHarness(t, "TWOPOD_MAX_ATTEMPTS=3", "TWOPOD_RETRY_DELAY=150ms")
	os.WriteFile(filepath.Join(h.dir, "mode-failsssssss"), []byte("fail"), 0600)
	failed := h.add("https://youtu.be/failsssssss")
	other := h.add("https://youtu.be/lmnopqrstuv")
	h.wait(other.ID, "published")
	failed = h.wait(failed.ID, "failed")
	if failed.Attempts != 3 {
		t.Fatalf("retry budget attempts=%d wanted3", failed.Attempts)
	}
	time.Sleep(200 * time.Millisecond)
	if h.status(failed.ID).Attempts != 3 {
		t.Fatal("retry exhaustion did not stop")
	}
	os.Remove(filepath.Join(h.dir, "mode-failsssssss"))
	h.run("retry", failed.ID)
	restored := h.wait(failed.ID, "published")
	if restored.ID != failed.ID || restored.Attempts != 1 {
		t.Fatalf("manual retry %+v", restored)
	}
	if out, err := h.command("retry", other.ID); err == nil {
		t.Fatalf("retried published source: %s", out)
	}
	rss, _ := h.feed()
	if len(rss.Channel.Items) != 2 {
		t.Fatal("retried source duplicated publication")
	}
}

func TestDeleteQueuedRunningPublishedAndRestore(t *testing.T) {
	h := newHarness(t)
	os.WriteFile(filepath.Join(h.dir, "mode-abcdefghijk"), []byte("wait"), 0600)
	running := h.add("https://youtu.be/abcdefghijk")
	h.wait(running.ID, "processing")
	queued := h.add("https://youtu.be/lmnopqrstuv")
	h.run("delete", queued.ID)
	if h.status(queued.ID).State != "deleted" {
		t.Fatal("queued deletion failed")
	}
	h.run("delete", running.ID)
	h.run("delete", running.ID)
	os.Remove(filepath.Join(h.dir, "mode-abcdefghijk"))
	time.Sleep(100 * time.Millisecond)
	rss, _ := h.feed()
	if len(rss.Channel.Items) != 0 || h.status(running.ID).State != "deleted" {
		t.Fatal("deleted extraction later published")
	}
	restored := h.add("https://youtube.com/watch?v=abcdefghijk")
	h.wait(restored.ID, "published")
	rss, _ = h.feed()
	if len(rss.Channel.Items) != 1 || rss.Channel.Items[0].GUID.Value != "youtube:abcdefghijk" {
		t.Fatal("restoration changed identity")
	}
	media := rss.Channel.Items[0].Enclosure.URL
	h.run("delete", restored.ID)
	rss, _ = h.feed()
	if len(rss.Channel.Items) != 0 {
		t.Fatal("published deletion retained feed")
	}
	r, err := http.Get(media)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 404 {
		t.Fatal("deleted media still available")
	}
	h.stop()
	h.start()
	time.Sleep(100 * time.Millisecond)
	rss, _ = h.feed()
	if len(rss.Channel.Items) != 0 || h.status(restored.ID).State != "deleted" {
		t.Fatal("restart resurrected deleted source")
	}
}

func TestCrashRecoveryStopsExtractionChildren(t *testing.T) {
	h := newHarness(t)
	prior := h.add("https://youtu.be/lmnopqrstuv")
	h.wait(prior.ID, "published")
	mode := filepath.Join(h.dir, "mode-abcdefghijk")
	os.WriteFile(mode, []byte("child"), 0600)
	running := h.add("https://youtu.be/abcdefghijk")
	h.wait(running.ID, "processing")
	deadline := time.Now().Add(3 * time.Second)
	var childPID int
	for time.Now().Before(deadline) {
		b, _ := os.ReadFile(mode + ".childpid")
		childPID, _ = strconv.Atoi(string(b))
		if childPID > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if childPID == 0 {
		t.Fatal("external child did not start")
	}
	defer syscall.Kill(childPID, syscall.SIGKILL)
	h.daemon.Process.Kill()
	h.daemon.Wait()
	h.daemon = nil
	os.Remove(mode)
	h.start()
	rss, _ := h.feed()
	if len(rss.Channel.Items) < 1 || h.status(prior.ID).State != "published" {
		t.Fatal("crash affected prior complete publication")
	}
	before, _ := os.ReadFile(mode + ".heartbeat")
	time.Sleep(150 * time.Millisecond)
	after, _ := os.ReadFile(mode + ".heartbeat")
	if !bytes.Equal(before, after) {
		t.Fatal("interrupted extractor child survived restart")
	}
	os.Remove(mode)
	h.wait(running.ID, "published")
	rss, _ = h.feed()
	if len(rss.Channel.Items) != 2 {
		t.Fatal("recovery lost/duplicated work")
	}
}

func TestExpiryIsFromPublicationAndRestoresStableIdentity(t *testing.T) {
	h := newHarness(t, "TWOPOD_RETENTION=700ms")
	s := h.add("https://youtu.be/abcdefghijk")
	h.wait(s.ID, "published")
	rss, _ := h.feed()
	media := rss.Channel.Items[0].Enclosure.URL
	mode := filepath.Join(h.dir, "mode-lmnopqrstuv")
	os.WriteFile(mode, []byte("wait"), 0600)
	other := h.add("https://youtu.be/lmnopqrstuv")
	h.wait(other.ID, "processing")
	time.Sleep(150 * time.Millisecond)
	req, _ := http.NewRequest("GET", media, nil)
	req.Header.Set("Range", "bytes=0-9")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 206 {
		t.Fatal("playback caused premature expiry")
	}
	h.wait(s.ID, "expired")
	rss, _ = h.feed()
	if len(rss.Channel.Items) != 0 {
		t.Fatal("expiry retained feed entry")
	}
	r, err = http.Get(media)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 404 {
		t.Fatal("expired media remains accessible")
	}
	if h.status(other.ID).State != "processing" {
		t.Fatal("retention started before publication")
	}
	os.Remove(mode)
	h.wait(other.ID, "published")
	restored := h.add("https://youtube.com/watch?v=abcdefghijk")
	h.wait(restored.ID, "published")
	rss, _ = h.feed()
	found := false
	for _, item := range rss.Channel.Items {
		if item.GUID.Value == "youtube:abcdefghijk" {
			found = true
		}
	}
	if !found {
		t.Fatal("resubmission changed stable identity")
	}
}

func TestStoragePausePreservesEpisodesAndResumesAfterDeletion(t *testing.T) {
	fixture, err := os.ReadFile("testdata/tone.mp3")
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, fmt.Sprintf("TWOPOD_STORAGE_LIMIT_BYTES=%d", 2*len(fixture)-1))
	prior := h.add("https://youtu.be/abcdefghijk")
	h.wait(prior.ID, "published")
	rss, _ := h.feed()
	media := rss.Channel.Items[0].Enclosure.URL
	queued := h.add("https://youtu.be/lmnopqrstuv")
	deadline := time.Now().Add(3 * time.Second)
	var paused submission
	for time.Now().Before(deadline) {
		paused = h.status(queued.ID)
		if paused.State == "queued" && paused.PausedReason != "" {
			break
		}
		if paused.State == "published" {
			t.Fatal("published beyond storage budget")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if paused.State != "queued" || paused.PausedReason == "" || paused.Attempts != 0 {
		t.Fatalf("storage did not pause without spending attempts: %+v", paused)
	}
	rss, _ = h.feed()
	if len(rss.Channel.Items) != 1 {
		t.Fatal("storage pressure discarded unexpired episode")
	}
	r, err := http.Get(media)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatal("storage pressure broke playback")
	}
	h.stop()
	h.start()
	time.Sleep(150 * time.Millisecond)
	paused = h.status(queued.ID)
	if paused.State != "queued" || paused.Attempts != 0 || paused.PausedReason == "" {
		t.Fatalf("restart lost storage pause: %+v", paused)
	}
	h.run("delete", prior.ID)
	h.wait(queued.ID, "published")
	rss, _ = h.feed()
	if len(rss.Channel.Items) != 1 || rss.Channel.Items[0].GUID.Value != "youtube:lmnopqrstuv" {
		t.Fatal("space release did not resume queued source")
	}
}

func TestSuccessfulExtractorMustProducePlayableMP3(t *testing.T) {
	h := newHarness(t, "TWOPOD_MAX_ATTEMPTS=1")
	os.WriteFile(filepath.Join(h.dir, "mode-abcdefghijk"), []byte("corrupt"), 0600)
	s := h.add("https://youtu.be/abcdefghijk")
	s = h.wait(s.ID, "failed")
	if s.Failure == "" {
		t.Fatal("unplayable audio lacked failure")
	}
	rss, _ := h.feed()
	if len(rss.Channel.Items) != 0 {
		t.Fatal("unplayable audio was published")
	}
	other := h.add("https://youtu.be/lmnopqrstuv")
	h.wait(other.ID, "published")
}

func TestGrowingWorkAndFreeSpacePauseWithoutLosingAcceptedSource(t *testing.T) {
	h := newHarness(t, "TWOPOD_STORAGE_LIMIT_BYTES=32768")
	mode := filepath.Join(h.dir, "mode-abcdefghijk")
	os.WriteFile(mode, []byte("grow"), 0600)
	s := h.add("https://youtu.be/abcdefghijk")
	deadline := time.Now().Add(4 * time.Second)
	var paused submission
	for time.Now().Before(deadline) {
		paused = h.status(s.ID)
		if paused.PausedReason != "" && paused.State == "queued" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if paused.State != "queued" || paused.Attempts != 0 || paused.PausedReason == "" {
		t.Fatalf("working-file pressure did not pause: %+v", paused)
	}
	rss, _ := h.feed()
	if len(rss.Channel.Items) != 0 {
		t.Fatal("temporary files were published")
	}
	h.stop()
	os.Remove(mode)
	h.env = append(h.env, "TWOPOD_STORAGE_LIMIT_BYTES=65536", "TWOPOD_MIN_FREE_BYTES=9223372036854775807")
	h.start()
	time.Sleep(100 * time.Millisecond)
	paused = h.status(s.ID)
	if paused.State != "queued" || paused.Attempts != 0 || paused.PausedReason == "" {
		t.Fatalf("free-space check failed: %+v", paused)
	}
	h.stop()
	h.env = append(h.env, "TWOPOD_MIN_FREE_BYTES=0")
	h.start()
	h.wait(s.ID, "published")
}

func TestConcurrentDedupAndDeleteAtCompletion(t *testing.T) {
	h := newHarness(t)
	mode := filepath.Join(h.dir, "mode-abcdefghijk")
	os.WriteFile(mode, []byte("wait"), 0600)
	results := make(chan []byte, 8)
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			out, err := h.command("add", "https://youtu.be/abcdefghijk?t=42")
			results <- out
			errors <- err
		}()
	}
	for i := 0; i < 8; i++ {
		out := <-results
		if err := <-errors; err != nil {
			t.Fatalf("concurrent add: %v %s", err, out)
		}
		var s submission
		if json.Unmarshal(out, &s) != nil || s.ID != "abcdefghijk" {
			t.Fatalf("dedup result %s", out)
		}
	}
	var list []submission
	if json.Unmarshal(h.run("list"), &list) != nil || len(list) != 1 {
		t.Fatalf("concurrent adds duplicated source: %+v", list)
	}
	h.wait("abcdefghijk", "processing")
	release := make(chan struct{})
	go func() { os.Remove(mode); close(release) }()
	h.run("delete", "abcdefghijk")
	<-release
	time.Sleep(100 * time.Millisecond)
	rss, _ := h.feed()
	if len(rss.Channel.Items) != 0 || h.status("abcdefghijk").State != "deleted" {
		t.Fatal("completion race resurrected deleted publication")
	}
}

func TestOverdueExpiryAndQueuedWorkRecoverOnRestart(t *testing.T) {
	h := newHarness(t, "TWOPOD_RETENTION=1s")
	prior := h.add("https://youtu.be/abcdefghijk")
	h.wait(prior.ID, "published")
	rss, _ := h.feed()
	media := rss.Channel.Items[0].Enclosure.URL
	mode := filepath.Join(h.dir, "mode-lmnopqrstuv")
	os.WriteFile(mode, []byte("wait"), 0600)
	running := h.add("https://youtu.be/lmnopqrstuv")
	h.wait(running.ID, "processing")
	queued := h.add("https://youtu.be/zyxwvutsrqp")
	h.stop()
	os.Remove(mode)
	time.Sleep(1100 * time.Millisecond)
	h.start()
	h.wait(prior.ID, "expired")
	h.wait(running.ID, "published")
	h.wait(queued.ID, "published")
	rss, _ = h.feed()
	if len(rss.Channel.Items) != 2 {
		t.Fatal("restart lost queued work or retained overdue episode")
	}
	r, err := http.Get(media)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 404 {
		t.Fatal("restart offered overdue audio")
	}
}

func TestProcessingTimeoutStopsChildrenAndAllowsNextSource(t *testing.T) {
	h := newHarness(t, "TWOPOD_PROCESS_TIMEOUT=400ms", "TWOPOD_MAX_ATTEMPTS=1")
	mode := filepath.Join(h.dir, "mode-abcdefghijk")
	os.WriteFile(mode, []byte("child"), 0600)
	s := h.add("https://youtu.be/abcdefghijk")
	other := h.add("https://youtu.be/lmnopqrstuv")
	s = h.wait(s.ID, "failed")
	if !strings.Contains(s.Failure, "timed out") {
		t.Fatalf("timeout outcome %+v", s)
	}
	h.wait(other.ID, "published")
	before, _ := os.ReadFile(mode + ".heartbeat")
	time.Sleep(100 * time.Millisecond)
	after, _ := os.ReadFile(mode + ".heartbeat")
	if !bytes.Equal(before, after) {
		t.Fatal("timeout left extraction child running")
	}
	rss, _ := h.feed()
	if len(rss.Channel.Items) != 1 {
		t.Fatal("timeout published partial audio or blocked next source")
	}
}

func TestGeneratedReadSecretPersistsWithoutManagementPrivileges(t *testing.T) {
	h := newHarness(t, "TWOPOD_READ_TOKEN=")
	url := strings.TrimSpace(string(h.run("feed-url")))
	parts := strings.Split(strings.TrimPrefix(url, h.base+"/"), "/")
	if len(parts) != 2 || len(parts[0]) != 64 || parts[1] != "feed.xml" {
		t.Fatal("generated read URL lacks strong secret")
	}
	r, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatal("generated read secret rejected")
	}
	request, _ := http.NewRequest("POST", strings.TrimSuffix(url, "feed.xml")+"add", strings.NewReader(`{"Action":"add","Argument":"https://youtu.be/abcdefghijk"}`))
	r, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 404 {
		t.Fatal("read secret authorized management")
	}
	var list []submission
	json.Unmarshal(h.run("list"), &list)
	if len(list) != 0 {
		t.Fatal("read-only request created work")
	}
	h.stop()
	h.start()
	if strings.TrimSpace(string(h.run("feed-url"))) != url {
		t.Fatal("restart changed generated read identity")
	}
}

func TestCleanRestartDoesNotSpendOnlyExtractionAttempt(t *testing.T) {
	h := newHarness(t, "TWOPOD_MAX_ATTEMPTS=1")
	mode := filepath.Join(h.dir, "mode-abcdefghijk")
	os.WriteFile(mode, []byte("wait"), 0600)
	s := h.add("https://youtu.be/abcdefghijk")
	h.wait(s.ID, "processing")
	h.stop()
	os.Remove(mode)
	h.start()
	s = h.wait(s.ID, "published")
	if s.Attempts != 1 {
		t.Fatalf("restart spent interrupted attempt: %+v", s)
	}
}

func TestApplicationAcceptanceIgnoresInheritedOperatorSettings(t *testing.T) {
	t.Setenv("TWOPOD_TELEGRAM_BOT_TOKEN", "production-token-placeholder")
	t.Setenv("TWOPOD_TELEGRAM_OPERATOR_ID", "invalid")
	t.Setenv("TWOPOD_TELEGRAM_API_URL", "https://invalid.example.invalid")
	t.Setenv("TWOPOD_RETENTION", "invalid")
	h := newHarness(t)
	s := h.add("https://youtu.be/abcdefghijk")
	h.wait(s.ID, "published")
}

func TestPartialMP3SuccessNeverPublishes(t *testing.T) {
	h := newHarness(t, "TWOPOD_MAX_ATTEMPTS=1")
	for _, example := range []struct{ id, mode string }{{"abcdefghijk", "partial"}, {"lmnopqrstuv", "wrongduration"}} {
		os.WriteFile(filepath.Join(h.dir, "mode-"+example.id), []byte(example.mode), 0600)
		s := h.add("https://youtu.be/" + example.id)
		s = h.wait(s.ID, "failed")
		if !strings.Contains(s.Failure, "complete") {
			t.Fatalf("incomplete audio failure unclear: %+v", s)
		}
		rss, _ := h.feed()
		if len(rss.Channel.Items) != 0 {
			t.Fatal("partial MP3 entered feed")
		}
		r, err := http.Get(h.base + "/" + strings.Repeat("s", 40) + "/media/" + s.ID + ".mp3")
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != 404 {
			t.Fatal("partial MP3 offered through media endpoint")
		}
	}
	full := h.add("https://youtu.be/zyxwvutsrqp")
	h.wait(full.ID, "published")
}

func TestExtractorProxyIsOptionalAndPassedOnlyInChildEnvironment(t *testing.T) {
	for _, proxy := range []string{"", "http://operator:proxy-password@127.0.0.1:8899"} {
		name := "direct"
		if proxy != "" {
			name = "configured"
		}
		t.Run(name, func(t *testing.T) {
			settings := []string{"TWOPOD_EXTRACTOR_PROXY=" + proxy, "FIXTURE_EXPECT_PROXY=" + proxy, "TWOPOD_MAX_ATTEMPTS=1", "http_proxy=", "https_proxy=", "HTTP_PROXY=", "HTTPS_PROXY=", "no_proxy=", "NO_PROXY="}
			if proxy != "" {
				settings = append(settings, "http_proxy=http://inherited.invalid:8888", "https_proxy=http://inherited.invalid:8888", "HTTP_PROXY=http://inherited.invalid:8888", "HTTPS_PROXY=http://inherited.invalid:8888", "no_proxy=youtube.com", "NO_PROXY=youtube.com")
			}
			h := newHarness(t, settings...)
			s := h.add("https://youtu.be/abcdefghijk")
			h.wait(s.ID, "published")
			rss, _ := h.feed()
			if len(rss.Channel.Items) != 1 {
				t.Fatal("proxy-aware extractor did not publish")
			}
			r, err := http.Get(rss.Channel.Items[0].Enclosure.URL)
			if err != nil {
				t.Fatal(err)
			}
			audio, _ := io.ReadAll(r.Body)
			r.Body.Close()
			fixture, _ := os.ReadFile("testdata/tone.mp3")
			if !bytes.Equal(audio, fixture) {
				t.Fatal("proxy-aware publication audio changed")
			}
			if strings.Contains(string(h.run("list")), "proxy-password") {
				t.Fatal("proxy credentials leaked to command output")
			}
			h.stop()
			if strings.Contains(h.log.String(), "proxy-password") {
				t.Fatal("proxy credentials leaked to daemon logs")
			}
		})
	}
}

func TestInvalidExtractorProxyIsRejectedWithoutCredentialDisclosure(t *testing.T) {
	for _, proxy := range []string{"http://operator:proxy-password@/", "http://operator:proxy-password@proxy.invalid:70000", "ftp://operator:proxy-password@proxy.invalid", "http://operator:proxy-password@proxy.invalid/path", "http://operator:proxy-password@proxy.invalid?token=secret", "http://operator:proxy-password@proxy.invalid#secret"} {
		cmd := exec.Command(binary, "list")
		for _, setting := range os.Environ() {
			if !strings.HasPrefix(setting, "TWOPOD_") {
				cmd.Env = append(cmd.Env, setting)
			}
		}
		cmd.Env = append(cmd.Env, "TWOPOD_EXTRACTOR_PROXY="+proxy)
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "TWOPOD_EXTRACTOR_PROXY") {
			t.Fatalf("invalid proxy did not produce configuration error: %s", out)
		}
		if strings.Contains(string(out), "proxy-password") || strings.Contains(string(out), "secret") {
			t.Fatal("invalid proxy credentials leaked")
		}
	}
}

func TestYouTubeBotChallengeHasSanitizedActionableFailure(t *testing.T) {
	h := newHarness(t, "TWOPOD_MAX_ATTEMPTS=1")
	os.WriteFile(filepath.Join(h.dir, "mode-abcdefghijk"), []byte("botchallenge"), 0600)
	s := h.add("https://youtu.be/abcdefghijk")
	s = h.wait(s.ID, "failed")
	if s.Failure != "YouTube blocked automated access from this network; configure an extraction proxy or retry later." {
		t.Fatalf("bot challenge diagnosis: %s", s.Failure)
	}
	if strings.Contains(string(h.run("list")), "proxy-password") {
		t.Fatal("extractor credentials leaked to command output")
	}
	rss, _ := h.feed()
	if len(rss.Channel.Items) != 0 {
		t.Fatal("blocked source entered feed")
	}
	r, err := http.Get(h.base + "/" + strings.Repeat("s", 40) + "/media/" + s.ID + ".mp3")
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 404 {
		t.Fatal("blocked source offered media")
	}
	h.stop()
	if strings.Contains(h.log.String(), "proxy-password") {
		t.Fatal("extractor credentials leaked to logs")
	}
}
