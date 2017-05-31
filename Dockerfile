FROM golang:1.8
EXPOSE 3456
RUN go get github.com/gorilla/mux
RUN go get github.com/lib/pq
RUN pwd && ls
CMD cd /go/src && ls && pwd && go run ./main.go
