// Fond — a self-hosted recipe keeper.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/au12/fond/internal/app"
	"github.com/au12/fond/web"
)

var version = "dev"

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string) bool {
	switch strings.ToLower(env(key, "")) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func envInt(key string, def int) int {
	if n, err := strconv.Atoi(env(key, "")); err == nil {
		return n
	}
	return def
}

// dropPrivileges lets the container start as root (so it can fix ownership of
// a freshly created /data mount) and then run as an unprivileged user, the
// way Unraid expects (PUID 99 / PGID 100). Started as non-root, it does nothing.
func dropPrivileges(dataDir string) {
	if os.Getuid() != 0 {
		return
	}
	uid, gid := envInt("PUID", 99), envInt("PGID", 100)
	if uid == 0 {
		return
	}
	os.MkdirAll(dataDir, 0o755)
	filepath.Walk(dataDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok && (int(st.Uid) != uid || int(st.Gid) != gid) {
			os.Chown(p, uid, gid)
		}
		return nil
	})
	if err := syscall.Setgroups([]int{gid}); err != nil {
		log.Fatalf("setgroups: %v", err)
	}
	if err := syscall.Setgid(gid); err != nil {
		log.Fatalf("setgid: %v", err)
	}
	if err := syscall.Setuid(uid); err != nil {
		log.Fatalf("setuid: %v", err)
	}
}

func parseProxies(v string) []*net.IPNet {
	var out []*net.IPNet
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "/") {
			if strings.Contains(part, ":") {
				part += "/128"
			} else {
				part += "/32"
			}
		}
		if _, n, err := net.ParseCIDR(part); err == nil {
			out = append(out, n)
		} else {
			log.Printf("ignoring invalid FOND_TRUSTED_PROXIES entry %q", part)
		}
	}
	return out
}

func main() {
	log.SetFlags(log.LstdFlags)
	dataDir := env("FOND_DATA", "/data")
	addr := env("FOND_ADDR", ":"+env("PORT", "8080"))

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "healthcheck":
			if err := app.Healthcheck(addr); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		case "version":
			fmt.Println(version)
			return
		case "reset-password":
			if len(os.Args) < 3 {
				fmt.Fprintln(os.Stderr, "usage: fond reset-password <username>")
				os.Exit(2)
			}
			dropPrivileges(dataDir)
			store, err := app.OpenStore(dataDir)
			if err != nil {
				log.Fatal(err)
			}
			raw := make([]byte, 12)
			rand.Read(raw)
			pw := base64.RawURLEncoding.EncodeToString(raw)
			if err := app.ResetPassword(store, os.Args[2], pw); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("New password for %s: %s\n", os.Args[2], pw)
			return
		case "reset-2fa":
			if len(os.Args) < 3 {
				fmt.Fprintln(os.Stderr, "usage: fond reset-2fa <username>")
				os.Exit(2)
			}
			dropPrivileges(dataDir)
			store, err := app.OpenStore(dataDir)
			if err != nil {
				log.Fatal(err)
			}
			if err := app.ResetTwoFactor(store, os.Args[2]); err != nil {
				log.Fatalf("no user named %q", os.Args[2])
			}
			fmt.Printf("Two-step sign-in is off for %s. They can sign in with their password and turn it on again.\n", os.Args[2])
			return
		}
	}

	dropPrivileges(dataDir)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("cannot create data directory %s: %v", dataDir, err)
	}
	store, err := app.OpenStore(dataDir)
	if err != nil {
		log.Fatalf("cannot open database in %s: %v (is the folder writable by uid %d?)", dataDir, err, os.Getuid())
	}
	defer store.Close()

	cfg := app.Config{
		DataDir:            dataDir,
		Addr:               addr,
		BaseURL:            env("FOND_BASE_URL", ""),
		TrustedProxies:     parseProxies(env("FOND_TRUSTED_PROXIES", "127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,::1/128,fc00::/7")),
		AllowPrivateImport: envBool("FOND_ALLOW_PRIVATE_IMPORT"),
		SecureCookies:      strings.ToLower(env("FOND_SECURE_COOKIES", "auto")),
		SessionTTL:         time.Duration(envInt("FOND_SESSION_DAYS", 90)) * 24 * time.Hour,
		HSTS:               envBool("FOND_HSTS"),
		AIBase:             strings.TrimRight(env("FOND_AI_BASE_URL", ""), "/"),
		AIModel:            env("FOND_AI_MODEL", ""),
		AIKey:              env("FOND_AI_API_KEY", ""),
		Version:            version,
	}
	srv, err := app.New(cfg, store, web.FS())
	if err != nil {
		log.Fatal(err)
	}
	httpSrv := srv.HTTPServer()
	go func() {
		log.Printf("Fond %s listening on %s (data in %s, uid %d)", version, addr, dataDir, os.Getuid())
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpSrv.Shutdown(ctx)
}
