# # Build stage
# FROM --platform=${TARGETPLATFORM} golang:1.21-alpine AS builder

# ARG TARGETPLATFORM
# ARG TARGETARCH
# ARG TARGETOS

# RUN apk add --no-cache git gcc musl-dev

# WORKDIR /app

# COPY . .

# # Install watchdog
# COPY --from=ghcr.io/openfaas/of-watchdog:0.9.11 /fwatchdog /usr/bin/fwatchdog
# RUN chmod +x /usr/bin/fwatchdog

# # Build the handler
# RUN CGO_ENABLED=1 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags "-s -w" -o handler .

# # Final stage
# FROM --platform=${TARGETPLATFORM} alpine:3.18

# RUN apk --no-cache add ca-certificates musl-dev \
#     && addgroup -S app && adduser -S -g app app


# WORKDIR /home/app

# COPY --from=builder --chown=app:app /app/handler    .
# COPY --from=builder --chown=app:app /app/data       ./data
# COPY --from=builder /usr/bin/fwatchdog              .
# COPY --from=builder --chown=app:app /app/data/facefinder /home/app/data/facefinder

# RUN chmod 644 /home/app/data/facefinder

# USER app

# ENV fprocess="./handler"
# ENV mode="http"
# ENV upstream_url="http://127.0.0.1:8082"

# CMD ["./fwatchdog"]

# Build stage
FROM --platform=${TARGETPLATFORM} golang:1.21-alpine AS builder

ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETOS

# Install required packages
RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

# Copy function files
COPY . .

# Install watchdog
COPY --from=ghcr.io/openfaas/of-watchdog:0.9.11 /fwatchdog /usr/bin/fwatchdog
RUN chmod +x /usr/bin/fwatchdog

# Build the handler
RUN CGO_ENABLED=1 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags "-s -w" -o handler .

# Final stage
FROM --platform=${TARGETPLATFORM} alpine:3.18

# Install required packages
RUN apk --no-cache add ca-certificates musl-dev \
    && addgroup -S app && adduser -S -g app app

WORKDIR /home/app

# Copy files from builder
COPY --from=builder --chown=app:app /app/handler    .
COPY --from=builder --chown=app:app /app/data       ./data
COPY --from=builder /usr/bin/fwatchdog              .
COPY --from=builder --chown=app:app /app/data/facefinder /home/app/data/facefinder

# Set permissions
RUN chmod 755 /home/app/handler \
    && chmod 644 /home/app/data/facefinder

USER app

# Set environment variables
ENV fprocess="./handler"
ENV mode="http"
ENV upstream_url="http://127.0.0.1:8082"
ENV prefix_logs="true"
ENV read_timeout="5m"
ENV write_timeout="5m"
ENV exec_timeout="5m"
ENV healthcheck_interval="5s"

CMD ["./fwatchdog"]