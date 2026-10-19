package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata"

	"wolfy/internal/wolfy"
)

func main() {
	if err := run(); err != nil {
		slog.Error("wolfy_stopped", "reason", err.Error())
		os.Exit(1)
	}
}
func run() error {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if command != "serve" {
		return maintenance(command)
	}
	config, err := wolfy.LoadConfig()
	if err != nil {
		return err
	}
	store, err := wolfy.OpenStore(config.DBPath)
	if err != nil {
		return err
	}
	defer store.Close()
	var directory wolfy.Directory
	var model wolfy.Model
	var messenger wolfy.Messenger
	if config.Mode == "demo" {
		directory = wolfy.DemoDirectory{}
		model = wolfy.DemoModel{}
		messenger = wolfy.DemoMessenger{}
	} else {
		directory, err = wolfy.NewSheets(config)
		if err != nil {
			return err
		}
		model = &wolfy.OpenAI{Client: wolfy.HTTPClient(), URL: "https://api.openai.com/v1/responses", APIKey: config.OpenAIKey, Model: config.Model}
		messenger = &wolfy.Meta{Client: wolfy.HTTPClient(), URL: "https://graph.facebook.com/" + config.GraphVersion + "/" + config.PhoneID + "/messages", Token: config.MetaToken}
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	worker := &wolfy.Worker{Store: store, Directory: directory, Model: model, Messenger: messenger, Config: config, Log: logger, Now: time.Now}
	webhook := &wolfy.Webhook{Store: store, Config: config, Now: time.Now}
	server := &http.Server{Addr: config.Listen, Handler: webhook.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	workerDone := make(chan error, 1)
	serverDone := make(chan error, 1)
	go func() { workerDone <- worker.Run(ctx) }()
	go func() { serverDone <- server.ListenAndServe() }()
	logger.Info("wolfy_started", "mode", config.Mode, "listen", config.Listen)
	var result error
	workerStopped, serverStopped := false, false
	select {
	case <-ctx.Done():
	case err := <-workerDone:
		result = err
		workerStopped = true
	case err := <-serverDone:
		if !errors.Is(err, http.ErrServerClosed) {
			result = err
		}
		serverStopped = true
	}
	cancel()
	shutdown, stop := context.WithTimeout(context.Background(), 15*time.Second)
	defer stop()
	if err := server.Shutdown(shutdown); err != nil {
		_ = server.Close()
	}
	if !workerStopped {
		if err := <-workerDone; result == nil {
			result = err
		}
	}
	if !serverStopped {
		if err := <-serverDone; result == nil && !errors.Is(err, http.ErrServerClosed) {
			result = err
		}
	}
	return result
}

func maintenance(command string) error {
	if command != "backup" && command != "check" && command != "cleanup" && command != "status" && command != "restore-safe" && command != "resume-ai" {
		return errors.New("usage: wolfy [serve|backup DESTINATION|check|cleanup|status|restore-safe|resume-ai SPENT_NANO_USD]; maintenance requires the service stopped")
	}
	path := os.Getenv("WOLFY_DB")
	if path == "" {
		path = "data/wolfy.db"
	}
	if _, err := os.Stat(path); err != nil {
		return errors.New("database does not exist")
	}
	store, err := wolfy.OpenStore(path)
	if err != nil {
		return err
	}
	defer store.Close()
	switch command {
	case "backup":
		if len(os.Args) != 3 {
			return errors.New("usage: wolfy backup DESTINATION")
		}
		if err := os.MkdirAll(filepath.Dir(os.Args[2]), 0700); err != nil {
			return err
		}
		if err := store.Cleanup(time.Now()); err != nil {
			return err
		}
		if err := store.Check(); err != nil {
			return err
		}
		return store.Backup(os.Args[2])
	case "restore-safe":
		return store.PrepareRestore(time.Now())
	case "resume-ai":
		if len(os.Args) != 3 {
			return errors.New("usage: wolfy resume-ai SPENT_NANO_USD")
		}
		spent, err := strconv.ParseInt(os.Args[2], 10, 64)
		if err != nil {
			return errors.New("invalid reconciled usage")
		}
		return store.ResumeAI(time.Now().UTC().Format("2006-01"), spent)
	case "cleanup":
		return store.Cleanup(time.Now())
	case "check":
		return store.Check()
	case "status":
		used, err := store.Used(time.Now().UTC().Format("2006-01"))
		if err != nil {
			return err
		}
		fmt.Printf("budget_used_nano_usd=%d\n", used)
		return nil
	}
	return nil
}
