# syntax=docker/dockerfile:1
FROM golang:1.23.0-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server .
RUN mkdir -p /out/data

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/server /server
COPY --from=build --chown=nonroot:nonroot /out/data /data
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
