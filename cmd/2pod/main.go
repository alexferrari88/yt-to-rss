package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/alexferrari88/yt-to-rss/service"
	"github.com/alexferrari88/yt-to-rss/telegram"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) > 1 && os.Args[1] == "_extract-worker" {
		return service.ExecExtractor(os.Args[2:])
	}
	cfg, err := service.ConfigFromEnv()
	if err != nil {
		return err
	}
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: 2pod serve | add URL | list | status [ID] | retry ID | delete ID | feed-url")
	}
	action := os.Args[1]
	if action == "serve" {
		if len(os.Args) != 2 {
			return fmt.Errorf("serve accepts no arguments")
		}
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()
		botConfig, err := telegram.ConfigFromEnv()
		if err != nil {
			return err
		}
		return service.Run(ctx, cfg, telegram.Adapter(botConfig))
	}
	argument := ""
	if len(os.Args) == 3 {
		argument = os.Args[2]
	}
	switch action {
	case "add", "retry", "delete":
		if argument == "" || len(os.Args) != 3 {
			return fmt.Errorf("%s requires one argument", action)
		}
	case "list", "feed-url":
		if len(os.Args) != 2 {
			return fmt.Errorf("%s accepts no arguments", action)
		}
	case "status":
		if len(os.Args) > 3 {
			return fmt.Errorf("status accepts at most one ID")
		}
	default:
		return fmt.Errorf("unknown command")
	}
	b, _ := json.Marshal(service.Command{Action: action, Argument: argument})
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", filepath.Join(cfg.StateDir, "control.sock"))
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second}
	response, err := client.Post("http://local/command", "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("cannot reach daemon; check that 2pod serve is running with the same state directory")
	}
	defer response.Body.Close()
	var decoded struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&decoded); err != nil {
		return fmt.Errorf("invalid daemon response")
	}
	if decoded.Error != "" {
		return fmt.Errorf("%s", decoded.Error)
	}
	if action == "feed-url" {
		var url string
		if json.Unmarshal(decoded.Result, &url) != nil {
			return fmt.Errorf("invalid daemon response")
		}
		fmt.Println(url)
	} else {
		fmt.Println(string(decoded.Result))
	}
	return nil
}
