package main

import (
	"fmt"
	"net/http"
	"os"
	"log"
	"log/slog"
	"time"
	"path/filepath"
	"context"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/bcrypt"
	"wertfrei.art/fileomat/core"
)

func main() {
	var err error

	if (len(os.Args) > 1) {
		// hack to display hash for a given password
		password := os.Args[1]
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			slog.Error(err.Error())
		}
		fmt.Println(string(hashedPassword))
		return
	}
	// load config
	core.Cfg, err = core.LoadConfig(filepath.Join("etc", "config.json"))
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	// setup logging
	file, err := os.OpenFile(filepath.Join("etc", core.Cfg.LogFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		slog.Error(err.Error())
	}
	defer file.Close()
	log.SetOutput(file)
	log.SetFlags(log.Ldate | log.Lmicroseconds)

	// string translations
	err = core.LoadTranslations(core.Cfg.Lang)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	// extra cleanup thread
	go core.Cleanup()

	// configure and start server
	http.HandleFunc("/", core.ReqHandler)

	server := &http.Server{Addr: ":" + core.Cfg.Port, Handler: http.DefaultServeMux}

	// setup a stop signal for mainthread
	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, syscall.SIGINT, syscall.SIGTERM)

	// start server inside an extra thread
	go func() {
		slog.Info("start http server. handle " + core.Cfg.BaseURL + " on port " + core.Cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error(err.Error())
		}
		slog.Info("server thread end")
	}()

	// mainthread waits for a signal
	<-stopSignal

	slog.Info("http server shutdown...")
	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()
	if err = server.Shutdown(ctx); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	// fine :-)
	slog.Info("http server stop.")
}
