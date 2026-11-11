FROM golang:1.27.4 AS build
WORKDIR /src
# Build context is the repo root (see template.yaml Metadata.DockerContext).
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -tags lambda.norpc -o /bootstrap ./functions/moderate

# No web adapter: S3 invokes this function directly with its event.
FROM public.ecr.aws/lambda/provided:al2023
COPY --from=build /bootstrap /var/task/bootstrap
ENTRYPOINT ["/var/task/bootstrap"]
