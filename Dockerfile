FROM golang:1.25-alpine AS builder

WORKDIR /app

ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.22

WORKDIR /app

ARG ALPINE_MIRROR=dl-cdn.alpinelinux.org
RUN if [ "$ALPINE_MIRROR" != "dl-cdn.alpinelinux.org" ]; then \
      sed -i "s/dl-cdn.alpinelinux.org/$ALPINE_MIRROR/g" /etc/apk/repositories; \
    fi
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/server ./server
COPY configs ./configs

EXPOSE 8850

CMD ["./server", "-config", "configs/config.yaml"]
