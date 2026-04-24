ssh ec2-user@18.142.132.182

cd /home/ec2-user/video-saas
git pull origin main

sudo bash infra/aws/scripts/bootstrap_start.sh /home/ec2-user/video-saas

然后检查状态：

docker compose \
  --project-name video-saas \
  --env-file /opt/video-saas/shared/env/global.env \
  -f /home/ec2-user/video-saas/docker-compose.bootstrap.yml \
  ps
