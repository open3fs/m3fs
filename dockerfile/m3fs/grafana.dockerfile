FROM grafana/grafana:12.0.0

RUN grafana cli plugins install grafana-clickhouse-datasource
