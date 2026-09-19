package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jdbnet/dockyard/internal/api"
	"github.com/jdbnet/dockyard/internal/config"
	"github.com/jdbnet/dockyard/internal/docker"
	"github.com/jdbnet/dockyard/internal/engine"
	"github.com/jdbnet/dockyard/internal/tui"
	"github.com/jdbnet/dockyard/internal/version"
)

func main() {
	web := flag.Bool("web", false, "enable HTTP/WebSocket server")
	headless := flag.Bool("headless", false, "run without TUI (requires --web)")
	debugList := flag.Bool("debug-list", false, "list containers and exit")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Version)
		return
	}

	configPath := "config.yaml"
	args := flag.Args()
	if len(args) > 0 {
		configPath = args[0]
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if *web {
		cfg.Web.Enabled = true
	}

	dc, err := docker.NewClient(cfg.Docker.Socket)
	if err != nil {
		log.Fatalf("docker: %v", err)
	}
	defer dc.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := dc.Ping(ctx); err != nil {
		log.Fatalf("cannot connect to docker at %s: %v\n(add user to docker group or check socket path)", cfg.Docker.Socket, err)
	}

	eng, err := engine.New(cfg, dc)
	if err != nil {
		log.Fatalf("engine: %v", err)
	}
	go func() {
		if err := eng.Run(ctx); err != nil && ctx.Err() == nil {
			log.Printf("engine: %v", err)
		}
	}()

	// Allow initial refresh
	time.Sleep(200 * time.Millisecond)

	if *debugList {
		runDebugList(ctx, eng)
		return
	}

	var srv *api.Server
	if cfg.Web.Enabled {
		if !cfg.WebBindIsLoopback() && !cfg.AuthEnabled() {
			log.Printf("warning: web UI is bound to %s without authentication; set auth.username and auth.password or bind to 127.0.0.1", cfg.Web.Bind)
		}
		srv = api.NewServer(cfg, eng, version.Version)
		go func() {
			log.Printf("web UI listening on http://%s", cfg.Addr())
			if err := srv.Start(); err != nil {
				log.Printf("http server: %v", err)
			}
		}()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	if !*headless {
		if err := tui.Run(ctx, eng, cfg, version.Version); err != nil {
			log.Printf("tui: %v", err)
		}
		cancel()
		if srv != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = srv.Shutdown(shutdownCtx)
			shutdownCancel()
		}
		eng.Shutdown()
		return
	}

	if !cfg.Web.Enabled {
		log.Fatal("headless mode requires --web")
	}

	log.Printf("dockyard %s running headless on http://%s", version.Version, cfg.Addr())
	<-sigCh
	cancel()
	if srv != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = srv.Shutdown(shutdownCtx)
		shutdownCancel()
	}
	eng.Shutdown()
}

func runDebugList(ctx context.Context, eng *engine.Engine) {
	projects, err := eng.ComposeProjects(ctx)
	if err != nil {
		log.Fatalf("list: %v", err)
	}
	for _, p := range projects {
		fmt.Printf("\n[%s]\n", p.Name)
		for _, c := range p.Containers {
			stack := c.ComposeProject
			if stack == "" {
				stack = "-"
			}
			fmt.Printf("  %-30s %-12s %-10s %s\n", c.Name, c.State, stack, c.Status)
		}
	}
}
