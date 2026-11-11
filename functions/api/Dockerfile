FROM golang:1.27.4 AS build
WORKDIR /src
# Build context is the repo root (see template.yaml Metadata.DockerContext),
# so the whole module - including internal/ and .git - is available here.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Version is derived from git rather than passed in as a build arg: SAM's
# Metadata.DockerBuildArgs only accepts static, literal values (no shell/env
# substitution), so there's no clean way to inject a computed value from
# outside. Computing it here works identically for local and CI builds as
# long as .git is present in the build context.
RUN apt-get update && apt-get install -y --no-install-recommends git && rm -rf /var/lib/apt/lists/*
RUN VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo dev) \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.Version=${VERSION}" -o /bootstrap ./functions/api

FROM public.ecr.aws/lambda/provided:al2023
COPY --from=public.ecr.aws/awsguru/aws-lambda-adapter:1.1.0 /lambda-adapter /opt/extensions/lambda-adapter
COPY --from=build /bootstrap /var/task/bootstrap
ENTRYPOINT ["/var/task/bootstrap"]
