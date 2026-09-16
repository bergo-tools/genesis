// Command genesis is a single-binary, agentic roleplay engine.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/auth"
	"github.com/zp/genesis/internal/config"
	"github.com/zp/genesis/internal/server"
	"github.com/zp/genesis/internal/store"
	"github.com/zp/genesis/internal/tools"
)

// version is overridable at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	addr := flag.String("addr", "", "listen address (overrides config)")
	dataDir := flag.String("data", "", "data directory holding config.json and llm_sessions")
	configPath := flag.String("config", "", "path to config.json (overrides -data)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("genesis", version)
		return
	}

	config.LoadDotEnv(".env")

	dd := firstNonEmpty(*dataDir, os.Getenv("GENESIS_DATA_DIR"), ".")
	cp := *configPath
	if cp == "" {
		cp = filepath.Join(dd, "config.json")
	}

	cfg, err := config.Load(cp)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if *dataDir != "" || *addr != "" {
		if err := cfg.Update(func(c *config.Config) {
			if *dataDir != "" {
				c.DataDir = *dataDir
			}
			if *addr != "" {
				c.Addr = *addr
			}
		}); err != nil {
			log.Fatalf("config: %v", err)
		}
	}

	c := cfg.Get()
	sessions, err := store.New(filepath.Join(c.DataDir, "sessions"))
	if err != nil {
		log.Fatalf("sessions: %v", err)
	}
	stories, err := store.NewStoryStore(filepath.Join(c.DataDir, "stories"))
	if err != nil {
		log.Fatalf("stories: %v", err)
	}
	if err := store.MigrateLegacySessions(sessions, stories); err != nil {
		log.Printf("migrate legacy sessions: %v", err)
	}
	if err := store.EnsureBuiltins(stories); err != nil {
		log.Fatalf("builtin stories: %v", err)
	}

	reg := agent.NewRegistry()
	tools.RegisterBuiltins(reg)

	authn := auth.New(os.Getenv("GENESIS_PASSWORD"))

	srv := server.New(cfg, sessions, stories, reg, authn)
	httpSrv := &http.Server{
		Addr:              c.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	url := "http://" + c.Addr
	if strings.HasPrefix(c.Addr, ":") {
		url = "http://127.0.0.1" + c.Addr
	}
	log.Printf("Genesis %s", version)
	log.Printf("  listening : %s", url)
	log.Printf("  tools     : %d registered", len(reg.Names()))
	log.Printf("  model     : %s", c.Model)
	log.Printf("  api key   : %s", keyState(c.APIKey))
	log.Printf("  password  : %s", authState(authn))
	log.Printf("  sessions  : %s", sessions.Dir())
	log.Printf("  stories   : %s", stories.Dir())
	log.Printf("  config    : %s", cp)

	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func keyState(key string) string {
	if strings.TrimSpace(key) == "" {
		return "not set (open Settings in the browser)"
	}
	return "set"
}

func authState(a *auth.Authenticator) string {
	if a != nil && a.Enabled() {
		return "set (GENESIS_PASSWORD, sessions kept in memory)"
	}
	return "not set (GENESIS_PASSWORD is empty, so the app is open)"
}
