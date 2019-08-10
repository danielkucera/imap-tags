package main

import (
	"log"
	"crypto/tls"

	"github.com/emersion/go-imap/server"
	"github.com/danielkucera/imap-tags/backend"
)

func main() {
	// Create a memory backend
	be := memory.New()

	// Create a new server
	s := server.New(be)
	s.Addr = ":993"

	cert, err := tls.LoadX509KeyPair("/etc/letsencrypt/live/danman.eu/fullchain.pem", "/etc/letsencrypt/live/danman.eu/privkey.pem")
	if err != nil {
		log.Fatal(err)
	}

	s.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}

	log.Printf("Starting IMAP server at %s", s.Addr)
	if err := s.ListenAndServeTLS(); err != nil {
		log.Fatal(err)
	}
}
