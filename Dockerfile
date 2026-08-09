# syntax=docker/dockerfile:1.7

FROM node:24-alpine AS web-builder
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --ignore-scripts --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS go-builder
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY webui/ ./webui/
COPY --from=web-builder /src/webui/dist/ ./webui/dist/

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o /out/gemini-web2api ./cmd/gateway && \
    mkdir -p /out/data && \
    chown 65532:65532 /out/data

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=go-builder /usr/share/zoneinfo/ /usr/share/zoneinfo/
COPY --from=go-builder --chown=65532:65532 /out/gemini-web2api /app/gemini-web2api
COPY --from=go-builder --chown=65532:65532 /out/data /data

ENV LISTEN_ADDR=:8080 \
    DATA_DIR=/data
VOLUME ["/data"]
EXPOSE 8080
USER 65532:65532
HEALTHCHECK --interval=30s --timeout=6s --start-period=12s --retries=3 \
  CMD ["/app/gemini-web2api", "healthcheck"]
ENTRYPOINT ["/app/gemini-web2api"]
CMD ["serve"]
