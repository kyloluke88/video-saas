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

func RunFFmpegWithTimeout(timeout time.Duration, args ...string) error {
	return runCommandWithTimeout(timeout, "ffmpeg", args...)
}

func RunFFmpegOutput(args ...string) (string, error) {
	return runCommandOutput("ffmpeg", args...)
}

func RunFFprobe(args ...string) (string, error) {
	return runCommandOutput("ffprobe", args...)
}

func runCommand(name string, args ...string) error {
	return runCommandWithTimeout(0, name, args...)
}

func runCommandWithTimeout(timeout time.Duration, name string, args ...string) error {
	out, err := runCommandRaw(name, timeout, args...)
	if err != nil {
		return fmt.Errorf("%s error: %w output=%s", name, err, out)
	}
	return nil
}

func runCommandOutput(name string, args ...string) (string, error) {
	return runCommandOutputWithTimeout(0, name, args...)
}

func runCommandOutputWithTimeout(timeout time.Duration, name string, args ...string) (string, error) {
	out, err := runCommandRaw(name, timeout, args...)
	return string(out), err
}

func resolveCommandTimeout(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	if configured := time.Duration(conf.Get[int]("worker.ffmpeg_timeout_sec")) * time.Second; configured > 0 {
		return configured
	}
	return 5 * time.Minute
}

func runCommandRaw(name string, timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), resolveCommandTimeout(timeout))
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}
