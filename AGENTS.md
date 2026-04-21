# AGENTS.md

## Project overview

This repository is a video SaaS product.
The system may include:
- frontend app
 - next.js
 - shadcnUI
 - tailwindcss
- backend API
 - golang
 - gin
- async jobs / workers
 - golang
 - MFA
 - FFmpeg
 - S3
 - Google AI studio
- file upload pipeline
- video rendering / ffmpeg pipeline
- AI generation pipeline
- billing / quota / usage logic

## Working rules

- Prefer narrow, localized changes over broad refactors.
- Do not read or summarize the whole repository unless explicitly requested.
- When fixing a bug, first identify the failing layer: frontend, API, queue, worker, storage, callback, or ffmpeg/render step.
- Before changing architecture, explain the minimal viable plan.
- Preserve existing API contracts unless the task explicitly allows breaking changes.
- Avoid introducing new dependencies unless necessary.

## Repository expectations

- Run the most relevant lint/test/check commands after edits when possible.
- For backend changes, inspect request/response types and error handling.
- For worker changes, inspect retry logic, idempotency, and status transitions.
- For ffmpeg/render changes, inspect command construction, input paths, output paths, exit codes, and stderr.
- For upload/storage changes, inspect signed URLs, MIME types, file size limits, and callback status updates.

## File handling strategy

- Start from the smallest relevant set of files.
- Prefer reading only the directories related to the current task.
- Avoid loading large generated files, lockfiles, build artifacts, and logs unless needed.

## When debugging

- Reproduce the issue first.
- Find the exact failing step.
- Explain root cause briefly.
- Then propose the smallest safe fix.

## Output style

- Be concise.
- State assumptions clearly.
- Show changed files first when the change spans multiple files.


## 其他
### TTS请求策略
- 1. 以block为单位去请求
- 2. 每个block之间会插入gap
```
读 PODCAST_BLOCK_GAP_MS
代码位置：/Users/luca/go/github.com/luca/video-saas/worker/services/podcast/audio/generate.go:986
调用位置：/Users/luca/go/github.com/luca/video-saas/worker/services/podcast/audio/generate_google_stages.go:85
```
