FROM golang:1.14 AS builder

RUN mkdir /pkgsite

WORKDIR /pkgsite

COPY . /pkgsite

RUN CGO_ENABLED=0 go build -o=frontend cmd/frontend/main.go
RUN CGO_ENABLED=0 go build -o=worker cmd/worker/main.go

FROM busybox

RUN mkdir /pkgsite

WORKDIR /pkgsite

COPY --from=builder /pkgsite/content /pkgsite/content

COPY --from=builder /pkgsite/frontend /pkgsite

COPY --from=builder /pkgsite/worker /pkgsite

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder  /usr/share/zoneinfo /usr/share/zoneinfo

CMD ["/pkgsite/frontend", "-proxy_url", "https://proxy.golang.org", "-direct_proxy"]
