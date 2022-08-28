# syntax=docker/dockerfile:1.4

FROM nginx:1.23

ARG VERSION

COPY agent/hack/repository/nginx.conf /etc/nginx/conf.d/default.conf
COPY _build/${VERSION}/bin /usr/share/nginx/html/repository/${VERSION}
COPY _build/${VERSION}/bin /usr/share/nginx/html/repository/latest
