package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Submission struct {
	ID            string
	SourceURL     string
	State         string
	Title         string
	Uploader      string
	Duration      float64
	Attempts      int
	Failure       string
	PausedReason  string
	PublishedAt   *time.Time
	NextAttemptAt *time.Time
	Bytes         int64
	Generation    int64 `json:"-"`
	RequiredBytes int64 `json:"-"`
}
type Service struct {
	cfg        Config
	db         *sql.DB
	mu         sync.Mutex
	activeID   string
	cancel     context.CancelFunc
	activeDone chan struct{}
}

func Run(ctx context.Context, cfg Config, adapters ...func(context.Context, *Service) error) error {
	if err := os.MkdirAll(cfg.StateDir, 0700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	if err := os.Chmod(cfg.StateDir, 0700); err != nil {
		return fmt.Errorf("secure state directory: %w", err)
	}
	lock, err := os.OpenFile(filepath.Join(cfg.StateDir, "service.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("another daemon owns this state directory")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	if cfg.ReadToken == "" {
		b, err := os.ReadFile(filepath.Join(cfg.StateDir, "read-token"))
		if os.IsNotExist(err) {
			b = make([]byte, 32)
			if _, err = rand.Read(b); err != nil {
				return err
			}
			b = []byte(hex.EncodeToString(b))
			if err = os.WriteFile(filepath.Join(cfg.StateDir, "read-token"), b, 0600); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		cfg.ReadToken = strings.TrimSpace(string(b))
		if !regexp.MustCompile(`^[A-Za-z0-9_-]{32,}$`).MatchString(cfg.ReadToken) {
			return fmt.Errorf("stored read token is invalid")
		}
	}
	for _, dir := range []string{"media", "work"} {
		if err := os.MkdirAll(filepath.Join(cfg.StateDir, dir), 0700); err != nil {
			return err
		}
	}
	db, err := sql.Open("sqlite3", "file:"+filepath.Join(cfg.StateDir, "state.sqlite")+"?_busy_timeout=5000&_journal_mode=WAL&_synchronous=FULL&_foreign_keys=on")
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS submissions(id TEXT PRIMARY KEY,source TEXT NOT NULL,state TEXT NOT NULL,title TEXT NOT NULL DEFAULT '',uploader TEXT NOT NULL DEFAULT '',duration REAL NOT NULL DEFAULT 0,attempts INTEGER NOT NULL DEFAULT 0,failure TEXT NOT NULL DEFAULT '',published INTEGER NOT NULL DEFAULT 0,eligible INTEGER NOT NULL DEFAULT 0,bytes INTEGER NOT NULL DEFAULT 0,generation INTEGER NOT NULL DEFAULT 1,created INTEGER NOT NULL); CREATE TABLE IF NOT EXISTS adapter_state(key TEXT PRIMARY KEY,value BLOB NOT NULL);`)
	if err != nil {
		return fmt.Errorf("initialize state: %w", err)
	}
	s := &Service{cfg: cfg, db: db}
	for _, statement := range []string{`ALTER TABLE submissions ADD COLUMN paused TEXT NOT NULL DEFAULT ''`, `ALTER TABLE submissions ADD COLUMN required INTEGER NOT NULL DEFAULT 0`} {
		if _, err := db.Exec(statement); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("upgrade storage state: %w", err)
		}
	}
	if err := stopAbandonedWorkers(filepath.Join(cfg.StateDir, "work")); err != nil {
		return err
	}
	if _, err = db.Exec(`UPDATE submissions SET state='queued',attempts=MAX(0,attempts-1) WHERE state='processing'`); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(cfg.StateDir, "work")); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(cfg.StateDir, "work"), 0700); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE submissions SET state='deleted',bytes=0 WHERE state='deleting'`); err != nil {
		return err
	}
	rows, err := db.Query(`SELECT id FROM submissions WHERE state!='published'`)
	if err != nil {
		return err
	}
	var withdrawn []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		withdrawn = append(withdrawn, id)
	}
	rows.Close()
	for _, id := range withdrawn {
		if err := os.Remove(filepath.Join(cfg.StateDir, "media", id+".mp3")); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	published, err := s.List(ctx)
	if err != nil {
		return err
	}
	for _, v := range published {
		if v.State != "published" {
			continue
		}
		info, statErr := os.Stat(filepath.Join(cfg.StateDir, "media", v.ID+".mp3"))
		if statErr != nil || !info.Mode().IsRegular() || info.Size() != v.Bytes {
			if _, err := db.Exec(`UPDATE submissions SET state='failed',bytes=0,failure='Published audio is missing or damaged; check persistent storage and retry.' WHERE id=?`, v.ID); err != nil {
				return err
			}
		}
	}
	socket := filepath.Join(cfg.StateDir, "control.sock")
	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		return err
	}
	control, err := net.Listen("unix", socket)
	if err != nil {
		return fmt.Errorf("listen on management socket: %w", err)
	}
	defer os.Remove(socket)
	if err := os.Chmod(socket, 0600); err != nil {
		control.Close()
		return err
	}
	reader, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		control.Close()
		return fmt.Errorf("listen for podcast reads: %w", err)
	}
	httpServer := &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	controlServer := &http.Server{Handler: http.HandlerFunc(s.command), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 10 * time.Second}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, len(adapters)+4)
	var wg sync.WaitGroup
	launch := func(f func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e := f(); e != nil && !errors.Is(e, http.ErrServerClosed) && !errors.Is(e, context.Canceled) {
				errCh <- e
			}
		}()
	}
	launch(func() error { return httpServer.Serve(reader) })
	launch(func() error { return controlServer.Serve(control) })
	launch(func() error { s.worker(ctx); return nil })
	launch(func() error { s.maintenance(ctx); return nil })
	for _, adapter := range adapters {
		adapter := adapter
		launch(func() error { return adapter(ctx, s) })
	}
	select {
	case <-ctx.Done():
		err = nil
	case err = <-errCh:
	}
	cancel()
	httpServer.Close()
	controlServer.Close()
	wg.Wait()
	return err
}

func canonical(raw string) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.User != nil || u.Port() != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", "", fmt.Errorf("provide an individual public or unlisted YouTube video URL")
	}
	var id string
	switch strings.ToLower(u.Hostname()) {
	case "youtu.be", "www.youtu.be":
		id = strings.TrimPrefix(u.Path, "/")
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com":
		if u.Path == "/watch" {
			id = u.Query().Get("v")
		} else {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "embed" || parts[0] == "live") {
				id = parts[1]
			}
		}
	default:
		return "", "", fmt.Errorf("only individual YouTube video URLs are supported")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`).MatchString(id) {
		return "", "", fmt.Errorf("YouTube URL must identify one video; playlists are unsupported")
	}
	return id, "https://www.youtube.com/watch?v=" + id, nil
}
func (s *Service) Add(ctx context.Context, raw string) (Submission, error) {
	id, source, err := canonical(raw)
	if err != nil {
		return Submission{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.db.ExecContext(ctx, `INSERT INTO submissions(id,source,state,created) VALUES(?,?,'queued',?) ON CONFLICT(id) DO NOTHING`, id, source, time.Now().UnixNano())
	if err != nil {
		return Submission{}, fmt.Errorf("could not persist submission")
	}
	_, err = s.db.ExecContext(ctx, `UPDATE submissions SET state='queued',attempts=0,failure='',eligible=0,published=0,bytes=0,paused='',required=0,generation=generation+1,created=? WHERE id=? AND state IN ('deleted','expired')`, time.Now().UnixNano(), id)
	if err != nil {
		return Submission{}, fmt.Errorf("could not restore submission")
	}
	return s.status(ctx, id)
}

const columns = `id,source,state,title,uploader,duration,attempts,failure,published,eligible,bytes,generation,paused,required`

func scan(row interface{ Scan(...any) error }) (Submission, error) {
	var v Submission
	var pub, next int64
	err := row.Scan(&v.ID, &v.SourceURL, &v.State, &v.Title, &v.Uploader, &v.Duration, &v.Attempts, &v.Failure, &pub, &next, &v.Bytes, &v.Generation, &v.PausedReason, &v.RequiredBytes)
	if pub != 0 {
		t := time.Unix(0, pub).UTC()
		v.PublishedAt = &t
	}
	if next != 0 {
		t := time.Unix(0, next).UTC()
		v.NextAttemptAt = &t
	}
	return v, err
}
func (s *Service) status(ctx context.Context, id string) (Submission, error) {
	v, err := scan(s.db.QueryRowContext(ctx, `SELECT `+columns+` FROM submissions WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return v, fmt.Errorf("submission not found")
	}
	if err != nil {
		return v, fmt.Errorf("could not read submission")
	}
	return v, nil
}
func (s *Service) Status(ctx context.Context, id string) (Submission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status(ctx, id)
}
func (s *Service) List(ctx context.Context) ([]Submission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.list(ctx)
}
func (s *Service) list(ctx context.Context) ([]Submission, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+columns+` FROM submissions ORDER BY created DESC,id`)
	if err != nil {
		return nil, fmt.Errorf("could not list submissions")
	}
	defer rows.Close()
	out := []Submission{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("could not read submissions")
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) LoadState(ctx context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b []byte
	err := s.db.QueryRowContext(ctx, `SELECT value FROM adapter_state WHERE key=?`, key).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return b, err
}
func (s *Service) SaveState(ctx context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT INTO adapter_state(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (s *Service) Retry(ctx context.Context, id string) (Submission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.status(ctx, id)
	if err != nil {
		return v, err
	}
	if v.State != "failed" {
		return v, fmt.Errorf("only failed submissions can be retried")
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE submissions SET state='queued',attempts=0,failure='',eligible=0,paused='',required=0,generation=generation+1 WHERE id=?`, id); err != nil {
		return v, fmt.Errorf("could not persist retry")
	}
	return s.status(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id string) (Submission, error) {
	s.mu.Lock()
	v, err := s.status(ctx, id)
	if err != nil {
		s.mu.Unlock()
		return v, err
	}
	if v.State == "deleted" {
		s.mu.Unlock()
		return v, nil
	}
	if v.State != "deleting" {
		if _, err := s.db.ExecContext(ctx, `UPDATE submissions SET state='deleting',generation=generation+1,failure='',eligible=0,paused='',required=0 WHERE id=?`, id); err != nil {
			s.mu.Unlock()
			return v, fmt.Errorf("could not persist deletion")
		}
	}
	var done chan struct{}
	if s.activeID == id && s.cancel != nil {
		s.cancel()
		done = s.activeDone
	}
	s.mu.Unlock()
	if done != nil {
		select {
		case <-ctx.Done():
			return v, fmt.Errorf("deletion is pending; check status again")
		case <-done:
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.status(ctx, id)
	if err != nil {
		return current, err
	}
	if current.State != "deleting" {
		return current, nil
	}
	if err := os.Remove(filepath.Join(s.cfg.StateDir, "media", id+".mp3")); err != nil && !os.IsNotExist(err) {
		return current, fmt.Errorf("audio cleanup failed; check permissions and repeat delete")
	}
	if err := os.RemoveAll(filepath.Join(s.cfg.StateDir, "work", fmt.Sprintf("%s-%d", id, current.Generation-1))); err != nil {
		return current, fmt.Errorf("working-file cleanup failed; check permissions and repeat delete")
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE submissions SET state='deleted',bytes=0 WHERE id=? AND state='deleting'`, id); err != nil {
		return current, fmt.Errorf("could not persist completed deletion")
	}
	return s.status(ctx, id)
}

func (s *Service) worker(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		s.mu.Lock()
		rows, err := s.db.Query(`SELECT `+columns+` FROM submissions WHERE state='queued' AND eligible<=? ORDER BY created,id`, time.Now().UnixNano())
		var candidates []Submission
		if err == nil {
			for rows.Next() {
				v, e := scan(rows)
				if e == nil {
					candidates = append(candidates, v)
				}
			}
			rows.Close()
		}
		var v Submission
		err = sql.ErrNoRows
		for _, candidate := range candidates {
			if reason, _ := s.storageReason(candidate.RequiredBytes); reason != "" {
				s.db.Exec(`UPDATE submissions SET paused=? WHERE id=?`, reason, candidate.ID)
				continue
			}
			v = candidate
			_, err = s.db.Exec(`UPDATE submissions SET state='processing',attempts=attempts+1,paused='' WHERE id=?`, v.ID)
			v.Attempts++
			break
		}
		s.mu.Unlock()
		if err == nil {
			s.extract(ctx, v)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

type metadata struct {
	Title        string  `json:"title"`
	Uploader     string  `json:"uploader"`
	Duration     float64 `json:"duration"`
	IsLive       bool    `json:"is_live"`
	Availability string  `json:"availability"`
}

func (s *Service) extract(ctx context.Context, v Submission) {
	defer func() {
		if ctx.Err() != nil {
			s.mu.Lock()
			s.db.Exec(`UPDATE submissions SET state='queued',attempts=MAX(0,attempts-1),failure='',eligible=0 WHERE id=? AND generation=? AND state IN ('processing','failed','queued')`, v.ID, v.Generation)
			s.mu.Unlock()
		}
	}()
	work := filepath.Join(s.cfg.StateDir, "work", fmt.Sprintf("%s-%d", v.ID, v.Generation))
	if err := os.MkdirAll(work, 0700); err != nil {
		s.finishFailure(v, "Cannot create working files; check state directory permissions.")
		return
	}
	defer os.RemoveAll(work)
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		s.finishFailure(v, "Cannot create extraction identity; retry.")
		return
	}
	token := hex.EncodeToString(tokenBytes)
	if err := os.WriteFile(filepath.Join(work, "worker-token"), []byte(token), 0600); err != nil {
		s.finishFailure(v, "Cannot persist extraction identity; check state directory permissions.")
		return
	}
	processCtx, cancel := context.WithTimeout(ctx, s.cfg.ProcessTimeout)
	defer cancel()
	s.mu.Lock()
	current, readErr := s.status(ctx, v.ID)
	if readErr != nil || current.State != "processing" || current.Generation != v.Generation {
		s.mu.Unlock()
		return
	}
	s.activeID = v.ID
	s.cancel = cancel
	s.activeDone = make(chan struct{})
	s.mu.Unlock()
	defer func() {
		stopAbandonedWorkers(filepath.Join(s.cfg.StateDir, "work"))
		os.RemoveAll(work)
		s.mu.Lock()
		s.activeID = ""
		s.cancel = nil
		close(s.activeDone)
		s.mu.Unlock()
	}()
	args := []string{"--ignore-config", "--no-playlist", "--no-progress", "--no-warnings", "--quiet", "--no-cache-dir", "--extract-audio", "--audio-format", "mp3", "--write-info-json", "--match-filter", "!is_live & availability != private & availability != subscriber_only & availability != premium_only", "-o", filepath.Join(work, "audio.%(ext)s")}
	if s.cfg.FFmpegLocation != "" {
		args = append(args, "--ffmpeg-location", s.cfg.FFmpegLocation)
	}
	args = append(args, "--", v.SourceURL)
	reason, fileLimit := s.storageReason(0)
	if reason != "" {
		s.pause(v, storageStop{reason: reason, required: 1})
		return
	}
	executable, err := os.Executable()
	if err != nil {
		s.finishFailure(v, "Cannot locate extraction worker; restart the daemon.")
		return
	}
	cmd := childCommand(processCtx, executable, token, append([]string{"_extract-worker", strconv.FormatInt(fileLimit, 10), s.cfg.Extractor}, args...)...)
	stopMonitor := make(chan struct{})
	monitorDone := make(chan storageStop, 1)
	go s.monitorStorage(processCtx, cancel, work, stopMonitor, monitorDone)
	runErr := cmd.Run()
	close(stopMonitor)
	storageResult := <-monitorDone
	if reason, _ := s.storageReason(0); reason != "" && storageResult.reason == "" {
		need, _ := directoryBytes(work)
		storageResult = storageStop{reason: reason, required: need + 1}
	}
	if storageResult.reason != "" {
		s.pause(v, storageResult)
		return
	}
	if runErr != nil {
		reason := "Extraction failed; check yt-dlp/FFmpeg updates and YouTube availability, then retry."
		if errors.Is(processCtx.Err(), context.DeadlineExceeded) {
			reason = "Processing timed out; check the video length and processing timeout, then retry."
		}
		s.finishFailure(v, reason)
		return
	}
	metadataFile, err := os.Open(filepath.Join(work, "audio.info.json"))
	var meta metadata
	if err != nil {
		s.finishFailure(v, "Source unavailable or unsupported, or metadata missing; only public/unlisted non-live videos are supported. Check yt-dlp/FFmpeg and retry.")
		return
	}
	decodeErr := json.NewDecoder(io.LimitReader(metadataFile, 4<<20)).Decode(&meta)
	metadataFile.Close()
	if decodeErr != nil {
		s.finishFailure(v, "Extractor metadata is invalid or too large; check dependencies.")
		return
	}
	if meta.IsLive || (meta.Availability != "" && meta.Availability != "public" && meta.Availability != "unlisted") {
		s.finishFailure(v, "Unsupported source: active livestreams and login-dependent videos are not supported.")
		return
	}
	audio := filepath.Join(work, "audio.mp3")
	info, err := os.Stat(audio)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		s.finishFailure(v, "Extractor did not produce completed MP3 audio; check dependencies.")
		return
	}
	actualDuration, err := s.probeMP3(processCtx, audio, token, meta.Duration)
	if err != nil {
		s.finishFailure(v, "Completed audio is not a complete playable MP3; check FFmpeg/ffprobe and retry.")
		return
	}
	if meta.Duration <= 0 {
		meta.Duration = actualDuration
	}
	if err := syncFile(audio); err != nil {
		s.finishFailure(v, "Could not persist completed audio; check disk space and retry.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err = s.status(ctx, v.ID)
	if err != nil || current.State != "processing" || current.Generation != v.Generation || ctx.Err() != nil {
		return
	}
	target := filepath.Join(s.cfg.StateDir, "media", v.ID+".mp3")
	if err := os.Rename(audio, target); err != nil {
		s.failLocked(v, "Cannot publish audio; check disk space and permissions.")
		return
	}
	if err := syncFile(filepath.Join(s.cfg.StateDir, "media")); err != nil {
		os.Remove(target)
		s.failLocked(v, "Could not persist completed audio; check disk space and retry.")
		return
	}
	_, err = s.db.Exec(`UPDATE submissions SET state='published',title=?,uploader=?,duration=?,published=?,bytes=?,failure='',eligible=0,paused='',required=0 WHERE id=? AND generation=? AND state='processing'`, meta.Title, meta.Uploader, meta.Duration, time.Now().UnixNano(), info.Size(), v.ID, v.Generation)
	if err != nil {
		os.Remove(target)
		s.failLocked(v, "Could not persist publication; check disk space and retry.")
	}
}
func (s *Service) finishFailure(v Submission, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failLocked(v, reason)
}
func (s *Service) failLocked(v Submission, reason string) {
	state := "failed"
	eligible := int64(0)
	if v.Attempts < s.cfg.MaxAttempts && !strings.HasPrefix(reason, "Unsupported source:") {
		state = "queued"
		eligible = time.Now().Add(s.cfg.RetryDelay).UnixNano()
	}
	s.db.Exec(`UPDATE submissions SET state=?,failure=?,eligible=? WHERE id=? AND generation=? AND state='processing'`, state, reason, eligible, v.ID, v.Generation)
}
