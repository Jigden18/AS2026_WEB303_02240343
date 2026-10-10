# ---- Stage 1: build ----
FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY gen ./gen
COPY service-b ./service-b
RUN CGO_ENABLED=0 go build -o /out/app ./service-b

# ---- Stage 2: minimal runtime image ----
FROM alpine:3.20
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
