FROM alpine:3.22
RUN apk add --no-cache ca-certificates ffmpeg
WORKDIR /workspace
ENTRYPOINT ["/workspace/.runtime/integration.test"]
