FROM golang:1.26.4 AS build
WORKDIR /app

COPY go.work go.work.sum* ./
COPY shared/ shared/
COPY tui/ tui/
RUN mkdir -p api && echo 'module api\ngo 1.26' > api/go.mod

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/controltui ./tui/cmd/app

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/controltui /usr/local/bin/controltui
ENTRYPOINT ["controltui"]
