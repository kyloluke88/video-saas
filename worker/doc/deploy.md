## Web Bootstrap Deploy

这套步骤是给 `web` 基础栈用的，只会启动：

- `caddy`
- `frontend`
- `backend`
- `postgres`
- `redis`

不会启动 worker。

## 1. 连接到 EC2 实例

不要进 AWS CloudShell。

要么本地 SSH：

```bash
ssh -i video-saas.pem ec2-user@18.142.132.182
```

要么在 AWS EC2 控制台里用 `EC2 Instance Connect` 连到实例。

连上后先确认你真的在实例里：

```bash
whoami
hostname
pwd
```

预期：

- `whoami` 是 `ec2-user`
- `hostname` 类似 `ip-172-31-...`

## 2. 进入项目并更新代码

```bash
cd /home/ec2-user/video-saas
git pull origin main
```

## 3. 确认部署环境文件存在

```bash
ls /opt/video-saas/shared/env
```

至少要有：

- `global.env`
- `backend.env`
- `frontend.env`

## 4. 启动 web 基础栈

```bash
sudo bash infra/aws/scripts/bootstrap_start.sh /home/ec2-user/video-saas
```

说明：

- 这一步会重新 build `backend` 和 `frontend`
- 所以 `git pull` 下来的最新前后端代码会生效
- 如果机器规格小，这一步可能会慢

## 5. 检查容器状态

```bash
docker compose \
  --project-name video-saas \
  --env-file /opt/video-saas/shared/env/global.env \
  -f /home/ec2-user/video-saas/docker-compose.bootstrap.yml \
  ps
```

预期能看到：

- `caddy`
- `frontend`
- `backend`
- `postgres`
- `redis`

其中 `frontend` 和 `backend` 最好都进入 `healthy`。

## 6. 如果启动失败，先看日志

```bash
docker compose \
  --project-name video-saas \
  --env-file /opt/video-saas/shared/env/global.env \
  -f /home/ec2-user/video-saas/docker-compose.bootstrap.yml \
  logs --tail=200 backend frontend caddy
```

## 7. 如果构建卡住，先看资源

```bash
free -h
df -h
docker system df
```

如果实例内存太小，`backend` 的 Go build 或 `frontend` 的 Next build 可能会很慢，甚至让 SSH 变得不稳定。
