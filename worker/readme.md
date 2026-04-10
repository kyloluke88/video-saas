## 完整流程图
现在 worker 启动后，顺序是这样的：

connectAndPrepare()
conn := amqp.Dial(...)
开一个临时 ch
setupTopology(ch)
关闭这个临时 ch
返回 conn
然后：

consumer pool 启动多个 goroutine
每个 goroutine 再调用 conn.Channel()
得到自己的消费 ch
ch.Consume(...)
收到消息
调 dispatcher.HandleMessage(ch, msg)

## PostgreSQL 持久化

worker 已具备独立连接 PostgreSQL 的实现，使用的环境变量与 backend 保持一致：

- `DB_CONNECTION`
- `DB_HOST`
- `DB_PORT`
- `DB_DATABASE`
- `DB_USERNAME`
- `DB_PASSWORD`
- `DB_MAX_IDLE_CONNECTIONS`
- `DB_MAX_OPEN_CONNECTIONS`
- `DB_MAX_LIFE_SECONDS`

实现位置：

- 配置读取：[config/worker.go](/Users/luca/go/github.com/luca/video-saas/worker/config/worker.go)
- GORM 依赖：[go.mod](/Users/luca/go/github.com/luca/video-saas/worker/go.mod)
- PostgreSQL 持久化入口：[internal/persistence/postgres.go](/Users/luca/go/github.com/luca/video-saas/worker/internal/persistence/postgres.go)

说明：

- worker 当前通过 `gorm.io/gorm` 和 `gorm.io/driver/postgres` 直接写入：
  - `projects`
  - `podcast_script_pages`
