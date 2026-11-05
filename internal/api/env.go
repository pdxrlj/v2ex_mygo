package api

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/joho/godotenv"
)

// loadEnv loads the .env file once per process. Environment variables that
// are already set take precedence over values from the file.
var loadEnv = sync.OnceFunc(func() {
	path, ok := findEnvFile()
	if !ok {
		log.Printf("load .env: file not found")
		return
	}
	if err := godotenv.Load(path); err != nil {
		log.Printf("load .env: %v", err)
	}
})

// findEnvFile walks up from the working directory looking for a .env file,
// so code running from a package directory still finds the project root's
// .env.
func findEnvFile() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		path := filepath.Join(dir, ".env")
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
