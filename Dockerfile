FROM golang:1.19 AS build

WORKDIR /

COPY . .

RUN go mod download && go build -o /openstack-exporter .

FROM quay.io/openstack.kolla/prometheus-openstack-exporter:2023.1-ubuntu-jammy as openstack-exporter

COPY --from=build /openstack-exporter /opt/openstack-exporter/openstack-exporter


