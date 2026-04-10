package common

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	conf "worker/pkg/config"
)

func RunFFmpeg(args ...string) error {
	return RunFFmpegContext(context.Background(), args...)
}

func RunFFmpegContext(ctx context.Context, args ...string) error {
	return runCommand(ctx, "ffmpeg", args...)
}

func RunFFmpegWithTimeout(timeout time.Duration, args ...string) error {
	return RunFFmpegWithTimeoutContext(context.Background(), timeout, args...)
}

func RunFFmpegWithTimeoutContext(ctx context.Context, timeout time.Duration, args ...string) error {
	return runCommandWithTimeout(ctx, timeout, "ffmpeg", args...)
}

func RunFFmpegOutput(args ...string) (string, error) {
	return RunFFmpegOutputContext(context.Background(), args...)
}

func RunFFmpegOutputContext(ctx context.Context, args ...string) (string, error) {
	return runCommandOutput(ctx, "ffmpeg", args...)
}

func RunFFprobe(args ...string) (string, error) {
	return RunFFprobeContext(context.Background(), args...)
}

func RunFFprobeContext(ctx context.Context, args ...string) (string, error) {
	return runCommandOutput(ctx, "ffprobe", args...)
}

func runCommand(ctx context.Context, name string, args ...string) error {
	return runCommandWithTimeout(ctx, 0, name, args...)
}

func runCommandWithTimeout(ctx context.Context, timeout time.Duration, name string, args ...string) error {
	out, err := runCommandRaw(ctx, name, timeout, args...)
	if err != nil {
		return fmt.Errorf("%s error: %w output=%s", name, err, out)
	}
	return nil
}

func runCommandOutput(ctx context.Context, name string, args ...string) (string, error) {
	return runCommandOutputWithTimeout(ctx, 0, name, args...)
}

func runCommandOutputWithTimeout(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	out, err := runCommandRaw(ctx, name, timeout, args...)
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

func runCommandRaw(ctx context.Context, name string, timeout time.Duration, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, resolveCommandTimeout(timeout))
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}
