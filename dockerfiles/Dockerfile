FROM golang:1.26.5-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE=api
RUN CGO_ENABLED=0 go build -trimpath -o /app/server ./cmd/${SERVICE}

FROM alpine:3.22
RUN apk add --no-cache ca-certificates ffmpeg tzdata
WORKDIR /app
COPY --from=builder /app/server ./server
COPY config/kratos.yml ./config/kratos.yml
ENTRYPOINT ["./server"]
CMD ["-config", "config/kratos.yml"]
