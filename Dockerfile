# write a docker file for prometheus
# Path: Dockerfile
FROM prom/prometheus
ADD etc/prometheus.yml /etc/prometheus/prometheus.yml
EXPOSE 9090
VOLUME ["/etc/prometheus"]
ENTRYPOINT ["/bin/prometheus"]
CMD ["--config.file=/etc/prometheus/prometheus.yml"]



# docker run -d -p 9090:9090 --name prometheus \
# --add-host="host.docker.internal:host-gateway" \
# -v /home/long/goapp/src/rec53/etc/prometheus.yml:/etc/prometheus/prometheus.yml  \
# prom/prometheus --config.file=/etc/prometheus/prometheus.yml