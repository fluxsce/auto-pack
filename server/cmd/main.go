package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/audit"
	"auto-pack/server/internal/batchlock"
	wsbuild "auto-pack/server/internal/build"
	"auto-pack/server/internal/config"
	"auto-pack/server/internal/crypt"
	"auto-pack/server/internal/env"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/intake"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/job/runner"
	"auto-pack/server/internal/migrate"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/release"
	"auto-pack/server/internal/settings"
	"auto-pack/server/internal/store"
	"auto-pack/server/internal/token"
	"auto-pack/server/internal/user"
	"auto-pack/server/internal/web/mod"
	"auto-pack/server/internal/web/router"
	"auto-pack/server/internal/workspace"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	logFile, err := cfg.SetupLog()
	if err != nil {
		log.Fatal(err)
	}
	if logFile != nil {
		defer logFile.Close()
	}
	ctx := context.Background()
	st, err := store.Open(ctx, store.Config{Driver: cfg.DBDriver, Path: cfg.DBPath, DSN: cfg.DBDSN})
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	if path, err := migrate.Apply(ctx, st, cfg.DBDriver); err != nil {
		log.Fatalf("migrate %s: %v", path, err)
	}
	if err := user.SeedDefaultAdmin(ctx, st); err != nil {
		log.Fatal(err)
	}
	if err := workspace.SeedDefaultWorkspace(ctx, st); err != nil {
		log.Fatal(err)
	}

	cipher, err := crypt.NewFromPassphrase(cfg.CryptKey)
	if err != nil {
		log.Fatal(err)
	}
	dir := access.NewDirectory(st)
	guard := iam.NewGuard(dir)
	users := user.NewService(st, guard, dir)
	workspaces := workspace.NewService(st, guard, cfg.WorkspaceRoot)
	if err := workspaces.EnsureRoot(); err != nil {
		log.Fatal(err)
	}
	projects := project.NewService(st, guard)
	envs := env.NewService(st, guard, cipher)
	appSettings := settings.NewService(st, guard)
	aud := audit.NewService(st, guard)
	eng := runner.NewEngine(st, projects, envs, cfg.DataDir(), cfg.WorkspaceRoot, cfg.GitCloneDepth)
	eng.SetSettings(appSettings)
	jobs := job.NewService(st, guard, eng)
	eng.SetSubmit(jobs.SubmitSystem)
	gate := batchlock.New()
	releases := release.NewService(ctx, st, guard, projects, workspaces, envs, jobs, gate, cfg.DataDir(), cfg.WorkspaceRoot, cfg.GitCloneDepth)
	if err := releases.RecoverInterrupted(ctx); err != nil {
		log.Fatal(err)
	}
	builds := wsbuild.NewService(ctx, st, guard, projects, workspaces, jobs, releases, gate, cfg.DataDir(), cfg.WorkspaceRoot)
	if err := builds.RecoverInterrupted(ctx); err != nil {
		log.Fatal(err)
	}
	if err := eng.Start(ctx); err != nil {
		log.Fatal(err)
	}
	var intakeSvc *intake.Service
	if in := intake.Load(ctx, envs); in.Enabled {
		intakeSvc = intake.New(intake.Options{
			Store:         st,
			Projects:      projects,
			Envs:          envs,
			Jobs:          jobs,
			WorkspaceRoot: cfg.WorkspaceRoot,
			Retries:       in.Retries,
			Mailer:        eng,
		})
		q, err := intake.OpenMNS(in.MNS)
		if err != nil {
			log.Fatal(err)
		}
		intakeSvc.UseMNS(q)
		if err := intakeSvc.Start(ctx); err != nil {
			log.Fatal(err)
		}
		log.Printf("intake mns request=%s result=%s", in.MNS.RequestQueue, in.MNS.ResultQueue)
	}
	sched := job.NewScheduler(jobs, projects)
	if err := sched.Start(ctx); err != nil {
		log.Fatal(err)
	}

	h := router.Handler(mod.Deps{
		Tokens:     token.NewIssuer(cfg.JWTSecret),
		Users:      users,
		Workspaces: workspaces,
		Projects:   projects,
		Envs:       envs,
		Settings:   appSettings,
		Jobs:       jobs,
		Audit:      aud,
		Release:    releases,
		Build:      builds,
		Frontend:   cfg.FrontendPath,
		StartedAt:  time.Now(),
	})
	srv := &http.Server{Addr: cfg.Addr, Handler: h}
	go func() {
		log.Printf("auto-pack %s listening %s (db=%s)", config.AppVersion, cfg.Addr, st.Driver())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	stop, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	sched.Stop()
	if intakeSvc != nil {
		intakeSvc.Stop()
	}
	eng.Stop()
	_ = srv.Shutdown(stop)
}
