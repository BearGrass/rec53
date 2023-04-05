# rec53
recursive dns

hello world!

## TODLIST
* add some metric
* use make image to crate a docker image
* use start_image.sh to start a prometheus docker with specified yml and port

## bug
* query www.baidu.com txt will crash

## start prometheus
docker run -d -p 9090:9090 \
--name prometheus --add-host="host.docker.internal:host-gateway" \
-v /home/long/goapp/src/rec53/etc/prometheus.yml:/etc/prometheus/prometheus.yml  \
prom/prometheus --config.file=/etc/prometheus/prometheus.yml

* view prometheus : http://127.0.0.1:9090/graph
* view client: http://127.0.0.1:9999/metric
* docker visit vm: https://cloud.tencent.com/developer/article/2240955
* prometheus config: https://www.prometheus.wang/configuration/demo.html