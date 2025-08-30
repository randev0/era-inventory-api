# ---- build stage ----
    FROM golang:1.23-bullseye AS build
    WORKDIR /src
    
    # cache deps
    COPY go.mod go.sum ./
    RUN go mod download
    
    # copy the rest
    COPY . .
    
    # build the binary from cmd/api
    RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
        go build -o /out/app ./cmd/api
    
    # ---- runtime stage (small image) ----
    FROM gcr.io/distroless/base-debian12
    WORKDIR /app
    COPY --from=build /out/app /app/app
    ENV PORT=8080
    EXPOSE 8080
    CMD ["/app/app"]
    