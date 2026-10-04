package service

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func (s *Service) maintenance(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()
	for {
		s.expire()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (s *Service) expire() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Exec(`UPDATE submissions SET state='expired' WHERE state='published' AND published<=?`, time.Now().Add(-s.cfg.Retention).UnixNano())
	rows, err := s.db.Query(`SELECT id FROM submissions WHERE state='expired' AND bytes>0`)
	if err != nil {
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		if err := os.Remove(filepath.Join(s.cfg.StateDir, "media", id+".mp3")); err == nil || os.IsNotExist(err) {
			s.db.Exec(`UPDATE submissions SET bytes=0 WHERE id=? AND state='expired'`, id)
		}
	}
}

func syncFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync publication: %w", err)
	}
	return nil
}

func directoryBytes(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}
func (s *Service) storageSpace() (int64, int64, error) {
	media, err := directoryBytes(filepath.Join(s.cfg.StateDir, "media"))
	if err != nil {
		return 0, 0, err
	}
	work, err := directoryBytes(filepath.Join(s.cfg.StateDir, "work"))
	if err != nil {
		return 0, 0, err
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(s.cfg.StateDir, &stat); err != nil {
		return 0, 0, err
	}
	free := int64(stat.Bavail) * int64(stat.Bsize)
	return media + work, free, nil
}
func (s *Service) storageReason(need int64) (string, int64) {
	used, free, err := s.storageSpace()
	if err != nil {
		return "Storage paused: cannot inspect working/audio files or free space; check permissions.", 0
	}
	remaining := s.cfg.StorageLimitBytes - used
	freeBudget := free - s.cfg.MinFreeBytes
	if freeBudget < remaining {
		remaining = freeBudget
	}
	if remaining <= 0 || remaining < need {
		return "Storage paused: delete an episode, wait for expiry, or increase the storage budget/free-space allowance.", remaining
	}
	return "", remaining
}

type storageStop struct {
	reason   string
	required int64
}

func (s *Service) monitorStorage(ctx context.Context, cancel context.CancelFunc, work string, stop <-chan struct{}, done chan<- storageStop) {
	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			done <- storageStop{}
			return
		case <-stop:
			done <- storageStop{}
			return
		case <-ticker.C:
			if reason, _ := s.storageReason(0); reason != "" {
				need, _ := directoryBytes(work)
				cancel()
				done <- storageStop{reason: reason, required: need + 1}
				return
			}
		}
	}
}
func (s *Service) pause(v Submission, stop storageStop) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Exec(`UPDATE submissions SET state='queued',attempts=MAX(0,attempts-1),paused=?,required=MAX(required,?),eligible=0 WHERE id=? AND generation=? AND state='processing'`, stop.reason, stop.required, v.ID, v.Generation)
}
