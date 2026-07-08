FROM golang:1.26.4 AS build
WORKDIR /app

COPY go.work go.work.sum* ./
COPY shared/ shared/
COPY api/ api/
RUN mkdir -p tui && echo 'module tui\ngo 1.26' > tui/go.mod

RUN go mod download 
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./api/cmd/app

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
