package common

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	conf "worker/pkg/config"
)

func RunFFmpeg(args ...string) error {
	return runCommand("ffmpeg", args...)
}

func RunFFmpegOutput(args ...string) (string, error) {
	return runCommandOutput("ffmpeg", args...)
}

func RunFFprobe(args ...string) (string, error) {
	return runCommandOutput("ffprobe", args...)
}

func runCommand(name string, args ...string) error {
	out, err := runCommandRaw(name, args...)
	if err != nil {
		return fmt.Errorf("%s error: %w output=%s", name, err, out)
	}
	return nil
}

func runCommandOutput(name string, args ...string) (string, error) {
	out, err := runCommandRaw(name, args...)
	return string(out), err
}

func runCommandRaw(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(conf.Get[int]("worker.ffmpeg_timeout_sec"))*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}
