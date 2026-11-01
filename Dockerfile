# syntax=docker/dockerfile:1

# ---- Build stage -------------------------------------------------------------
FROM golang:1.25-alpine AS build

WORKDIR /src

# Download dependencies first so this layer is cached between code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X github.com/kthesword/github-trending-repos/cmd.version=${VERSION}" \
      -o /out/trending-repos .

# ---- Runtime stage -----------------------------------------------------------
# distroless/static includes CA certificates (needed for HTTPS to GitHub) and
# runs as a non-root user, with no shell or package manager.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/trending-repos /usr/local/bin/trending-repos

ENTRYPOINT ["/usr/local/bin/trending-repos"]
