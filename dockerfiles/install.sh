docker pull consul:latest
docker run -d --name consul --net=host consul agent -server -ui -node=server-1 -bootstrap-expect=1 -client=0.0.0.0 -node=main --bind=127.0.0.1
docker build -t nginx/consul:latest ./nginx
docker run -it --name nginx -v /home/consul/nginx/nginx.conf:/data/nginx/conf/nginx.conf -p 0.0.0.0:9000:80 -d nginx/consul:latest