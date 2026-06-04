package main

import (
	"log"
	"net/http"

	"auto-pack/backend/internal/app"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/config"
	"auto-pack/backend/pkg/db"
	"auto-pack/backend/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	if err := logger.Init(logger.FromConfig(
		cfg.Log.Level,
		cfg.Log.File,
		cfg.Log.MaxSizeMB,
		cfg.Log.MaxBackups,
		cfg.Log.MaxAgeDays,
		cfg.Log.AlsoStdout,
	)); err != nil {
		log.Fatalf("init logger failed: %v", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		logger.Fatal("init sqlite failed", "err", err)
	}
	defer database.Close()

	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: app.NewHandler(database, cfg),
	}

	adminUser, adminPass := repository.DefaultAdminCredentials()
	logger.Info("backend server listening", "url", "http://localhost"+cfg.Addr)
	logger.Info("default admin credentials (change after first login)",
		"username", adminUser, "password", adminPass)

	if err := server.ListenAndServe(); err != nil {
		logger.Fatal("server failed", "err", err)
	}
}
