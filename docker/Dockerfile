FROM ubuntu:18.04
EXPOSE 80/tcp
RUN mkdir -p /data/nginx/conf
ADD sources.list /etc/apt/sources.list
WORKDIR /data/nginx
RUN apt-get update
RUN apt-get install -y git wget build-essential libtool libpcre3 libpcre3-dev zlib1g-dev openssl
RUN wget http://nginx.org/download/nginx-1.14.2.tar.gz
RUN tar -zxvf nginx-1.14.2.tar.gz
RUN git clone https://mirror.ghproxy.com/https://github.com/xiaokai-wang/nginx_upstream_check_module.git
RUN git clone https://mirror.ghproxy.com/https://github.com/weibocom/nginx-upsync-module
WORKDIR /data/nginx/nginx-1.14.2
RUN patch -p1 < /data/nginx/nginx_upstream_check_module/check_1.12.1+.patch
RUN ./configure --prefix=/data/nginx --add-module=/data/nginx/nginx_upstream_check_module --add-module=/data/nginx/nginx-upsync-module
RUN make -j8 && make install
WORKDIR /data/nginx
ADD nginx.conf /data/nginx/conf/nginx.conf
ADD bootstrap.sh /home/bootstrap.sh
CMD ["/bin/bash","-c", "/home/bootstrap.sh"]