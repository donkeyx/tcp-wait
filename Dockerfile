# # multi stage build, yo!
FROM golang:1.18.2
COPY . /app/
WORKDIR /app
RUN make deps build
# RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o tcp-wait .

# switched for potential shell
FROM alpine:latest
WORKDIR /app
COPY --from=0 /app/bin/tcp-wait /app/tcp-wait
ENTRYPOINT [ "/app/tcp-wait" ]
