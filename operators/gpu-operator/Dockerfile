# Build stage
FROM golang:1.27-bookworm AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/

# Build
# go-nvml needs cgo (it dlopen()s libnvidia-ml at runtime), so link against glibc.
RUN CGO_ENABLED=1 go build -a -o manager main.go

# Runtime stage
FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /

COPY --from=builder /workspace/manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
