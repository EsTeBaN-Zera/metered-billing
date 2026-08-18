FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG PACKAGE=./cmd/api
RUN CGO_ENABLED=0 go build -o /out/app ${PACKAGE}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
