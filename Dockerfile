FROM golang:1.8
EXPOSE 3456
RUN go get github.com/gorilla/mux
RUN go get github.com/mattn/go-sqlite3
RUN pwd && ls
CMD ls && pwd && go run ./main.go
