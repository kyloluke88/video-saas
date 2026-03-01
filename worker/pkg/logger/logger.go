package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

func InitLogger(filename string) error {
	if filename == "" {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
		return nil
	}
	resolvedFilename := dailyFilename(filename)
	if err := os.MkdirAll(filepath.Dir(resolvedFilename), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(resolvedFilename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	log.SetOutput(io.MultiWriter(os.Stdout, f))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	return nil
}

func dailyFilename(filename string) string {
	dir := filepath.Dir(filename)
	if dir == "." {
		dir = ""
	}
	return filepath.Join(dir, time.Now().Format("2006-01-02.log"))
}
