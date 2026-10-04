package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type cappedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	length := len(p)
	remaining := b.limit - b.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		b.Buffer.Write(p)
	}
	return length, nil
}
func childCommand(ctx context.Context, executable, token string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Env = append(os.Environ(), "TWOPOD_WORKER_KEY="+token)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 2 * time.Second
	return cmd
}
func (s *Service) probeMP3(ctx context.Context, path, token string) (float64, error) {
	executable := "ffprobe"
	if s.cfg.FFmpegLocation != "" {
		info, err := os.Stat(s.cfg.FFmpegLocation)
		if err != nil {
			return 0, fmt.Errorf("FFmpeg location unavailable")
		}
		dir := s.cfg.FFmpegLocation
		if !info.IsDir() {
			dir = filepath.Dir(dir)
		}
		executable = filepath.Join(dir, "ffprobe")
	}
	cmd := childCommand(ctx, executable, token, "-v", "error", "-select_streams", "a:0", "-show_entries", "stream=codec_name:format=duration", "-of", "json", path)
	output := &cappedBuffer{limit: 8192}
	cmd.Stdout = output
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("MP3 validation failed")
	}
	var result struct {
		Streams []struct {
			Codec string `json:"codec_name"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if json.Unmarshal(output.Bytes(), &result) != nil || len(result.Streams) != 1 || result.Streams[0].Codec != "mp3" {
		return 0, fmt.Errorf("completed audio is not MP3")
	}
	duration, err := strconv.ParseFloat(result.Format.Duration, 64)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("completed MP3 has no playable duration")
	}
	return duration, nil
}

// ExecExtractor is the binary's internal child entry point. Its file-size limit
// follows yt-dlp into FFmpeg; only the daemon monitors the aggregate budget.
func ExecExtractor(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("invalid extraction worker arguments")
	}
	limit, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil || limit == 0 {
		return fmt.Errorf("invalid extraction file limit")
	}
	path, err := exec.LookPath(args[1])
	if err != nil {
		return fmt.Errorf("extractor executable unavailable")
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: limit, Max: limit}); err != nil {
		return fmt.Errorf("cannot constrain extraction files")
	}
	return syscall.Exec(path, append([]string{path}, args[2:]...), os.Environ())
}

// A private per-extraction marker lets recovery identify surviving descendants
// without trusting a reusable PID. yt-dlp and FFmpeg inherit this environment.
func stopAbandonedWorkers(workRoot string) error {
	entries, err := os.ReadDir(workRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		token, err := os.ReadFile(filepath.Join(workRoot, entry.Name(), "worker-token"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if len(token) != 32 {
			return fmt.Errorf("invalid interrupted-worker marker; inspect working files")
		}
		deadline := time.Now().Add(3 * time.Second)
		for {
			active := false
			processes, err := os.ReadDir("/proc")
			if err != nil {
				return fmt.Errorf("cannot inspect interrupted extractors")
			}
			for _, p := range processes {
				pid, err := strconv.Atoi(p.Name())
				if err != nil {
					continue
				}
				env, err := os.ReadFile(filepath.Join("/proc", p.Name(), "environ"))
				if err != nil || !bytes.Contains(append(env, 0), append([]byte("TWOPOD_WORKER_KEY="+string(token)), 0)) {
					continue
				}
				stat, err := os.ReadFile(filepath.Join("/proc", p.Name(), "stat"))
				if err != nil {
					continue
				}
				close := strings.LastIndex(string(stat), ")")
				if close < 0 {
					continue
				}
				fields := strings.Fields(string(stat)[close+1:])
				if len(fields) < 3 || fields[0] == "Z" {
					continue
				}
				group, err := strconv.Atoi(fields[2])
				if err != nil || group <= 1 || group == syscall.Getpgrp() {
					return fmt.Errorf("interrupted extractor has an unexpected process group")
				}
				active = true
				if err := syscall.Kill(-group, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
					return fmt.Errorf("cannot stop interrupted extractor %d", pid)
				}
			}
			if !active {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("interrupted extractor did not stop; restart after inspecting processes")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	return nil
}
