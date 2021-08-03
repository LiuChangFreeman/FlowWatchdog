docker pull consul:latest
docker run -d --name consul --net=host --restart=always consul agent -server -ui -node=server-1 -bootstrap-expect=1 -client=0.0.0.0 -node=main --bind=127.0.0.1
chmod +x docker/bootstrap.sh
docker build -t nginx/consul:latest ./docker
mkdir -p /home/consul/
cp docker/nginx.conf /home/consul/nginx.conf
docker run -it --name nginx -v /home/consul/nginx.conf:/data/nginx/conf/nginx.conf --net=host --restart=always -d nginx/consul:latest