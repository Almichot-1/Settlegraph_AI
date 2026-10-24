FROM golang:1.27.0-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
ARG TARGET
RUN CGO_ENABLED=0 go build -trimpath -o /service ./cmd/${TARGET}

FROM alpine:3.21
RUN addgroup -S app && adduser -S -G app app
USER app
COPY --from=build /service /service
ENTRYPOINT ["/service"]
