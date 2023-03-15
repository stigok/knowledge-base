# Start by building the application.
FROM golang:1.19 as build

WORKDIR /go/src/app
COPY go.mod go.sum ./
RUN go mod download -x

COPY . .
RUN CGO_ENABLED=0 go build -o /go/bin/app

# Now copy it into our base image.
FROM gcr.io/distroless/static-debian11
USER 1002:1002
COPY --from=build /go/bin/app /
CMD ["/app"]
