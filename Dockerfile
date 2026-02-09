# Build stage
FROM golang:1.22-alpine AS builder

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the operator binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o /goplatform \
    ./cmd/goplatform

# Runtime stage
FROM alpine:3.19

# Install ca-certificates and terraform
RUN apk add --no-cache ca-certificates tzdata

# Copy Terraform binary (will be installed in CI/build)
# For production, install terraform here or mount it
# RUN wget -O /tmp/terraform.zip https://releases.hashicorp.com/terraform/1.6.6/terraform_1.6.6_linux_amd64.zip && \
#     unzip /tmp/terraform.zip -d /usr/local/bin/ && \
#     rm /tmp/terraform.zip

# Create non-root user
RUN adduser -D -g '' goplatform

# Copy binary from builder
COPY --from=builder /goplatform /usr/local/bin/goplatform

# Use non-root user
USER goplatform

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/goplatform"]
