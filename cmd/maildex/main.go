// Command maildex is a tiny self-hosted catalogue of which service uses which email.
package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Bagareal/maildex/internal/server"
	"github.com/Bagareal/maildex/internal/store"
	"github.com/Bagareal/maildex/web"
)

// version is set at build time with -ldflags "-X main.version=v1.2.3".
var version = "dev"

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	dataDir := env("MAILDEX_DATA", "/var/lib/maildex")
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "version", "--version", "-v":
		fmt.Println(version)
	case "reset-password":
		if err := store.ResetAuth(dataDir); err != nil {
			log.Fatalf("reset failed: %v", err)
		}
		fmt.Println("Password, biometrics and sessions removed. Restart the service, then open the web UI to choose a new password.")
	case "serve":
		serve(dataDir)
	default:
		fmt.Fprintf(os.Stderr, "usage: maildex [serve|version|reset-password]\n")
		os.Exit(2)
	}
}

func serve(dataDir string) {
	st, err := store.Open(dataDir)
	if err != nil {
		log.Fatalf("opening data in %s: %v", dataDir, err)
	}
	addr := env("MAILDEX_ADDR", ":8080")
	srv := &http.Server{
		Addr:              addr,
		Handler:           server.New(st, version, web.Static()).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	cert, key := os.Getenv("MAILDEX_TLS_CERT"), os.Getenv("MAILDEX_TLS_KEY")
	log.Printf("Maildex %s listening on %s (data: %s)", version, addr, dataDir)
	if cert != "" && key != "" {
		err = srv.ListenAndServeTLS(cert, key)
	} else {
		err = srv.ListenAndServe()
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
