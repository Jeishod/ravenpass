//go:build android && !production

package main

import (
	"log"
	"os"
)

// frontendDevServerURL is linked in by the live-reload build: an Android process inherits no shell environment.
var frontendDevServerURL string

func init() {
	if frontendDevServerURL == "" {
		return
	}
	if err := os.Setenv("FRONTEND_DEVSERVER_URL", frontendDevServerURL); err != nil {
		log.Fatal(err)
	}
}
