# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server .

FROM alpine:3.21
RUN addgroup -S app && adduser -S app -G app \
    && mkdir -p /data/saves && chown -R app:app /data
WORKDIR /app
COPY --from=build /out/server /app/server
USER app
ENV CS_DATA_DIR=/data CS_PORT=8090
EXPOSE 8090
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD wget -qO- http://localhost:8090/api/health || exit 1
CMD ["/app/server"]
