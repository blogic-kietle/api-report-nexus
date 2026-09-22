FROM golang:alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /report-nexus ./cmd/api

# PDF rendering runs in the gotenberg container (docker-compose), so no Chromium or fonts here.
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
ENV PORT=8000
WORKDIR /app
COPY --from=build /report-nexus /app/report-nexus
USER app
EXPOSE 8000
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s CMD wget -qO- http://127.0.0.1:8000/healthz || exit 1
ENTRYPOINT ["/app/report-nexus"]
