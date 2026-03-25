# syntax=docker/dockerfile:1.7

FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/vaultline ./cmd/vaultline

FROM debian:bookworm-slim
RUN useradd -u 10001 -r vaultline
WORKDIR /app
COPY --from=build /out/vaultline /usr/local/bin/vaultline
RUN mkdir -p /var/lib/vaultline && chown vaultline:vaultline /var/lib/vaultline
VOLUME ["/var/lib/vaultline"]
EXPOSE 8428
USER vaultline
ENV VAULTLINE_STORE_DIR=/var/lib/vaultline
ENTRYPOINT ["vaultline","daemon","--addr","0.0.0.0:8428","--store-dir","/var/lib/vaultline"]
