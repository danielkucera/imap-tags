package main

import (
	"crypto/tls"
	"log"
	"os"

	"github.com/danielkucera/imap-tags/backend"
	"github.com/emersion/go-imap/server"
)

type Logger struct {
}

func (l Logger) Write(p []byte) (n int, err error) {
	log.Printf("%s", p)
	return len(p), nil
}

func main() {
	// Create a memory backend
	be := memory.New(os.Getenv("DB_CONN"))

	// Create a new server
	s := server.New(be)
	s.Addr = ":143"

	s.AllowInsecureAuth = true

	s.Debug = Logger{}

	cert, err := tls.LoadX509KeyPair("/etc/letsencrypt/live/danman.eu/fullchain.pem", "/etc/letsencrypt/live/danman.eu/privkey.pem")
	if err != nil {
		log.Fatal(err)
	}

	s.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}

	log.Printf("Starting IMAP server at %s", s.Addr)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
