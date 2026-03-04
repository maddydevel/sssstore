FROM golang:1.22 AS builder
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/sssstore ./cmd/sssstore

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/sssstore /usr/local/bin/sssstore
EXPOSE 9000
ENTRYPOINT ["/usr/local/bin/sssstore"]
CMD ["server", "--config", "/etc/sssstore/sssstore.json"]
