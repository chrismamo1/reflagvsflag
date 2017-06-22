FROM golang:1.8
EXPOSE 80
RUN go get github.com/gorilla/mux
RUN go get github.com/lib/pq
RUN go get github.com/chrismamo1/reflagvsflag
RUN apt-get update && apt-get install -y ruby
RUN gem install sass
RUN sass -v
RUN curl -sL https://deb.nodesource.com/setup_6.x | bash -
RUN sudo apt-get install -y nodejs
CMD cd / && \
        rm -rf /go/src/github.com/chrismamo1/reflagvsflag/* && \
        rm -rf /go/src/github.com/chrismamo1/reflagvsflag/.git/* && \
        rmdir /go/src/github.com/chrismamo1/reflagvsflag/.git && \
        rmdir /go/src/github.com/chrismamo1/reflagvsflag && \
        go get github.com/chrismamo1/reflagvsflag && \
        cd /go/src/github.com/chrismamo1/reflagvsflag && \
        ls && \
        pwd && \
        make static/js/reflagvsflag.js && \
        make styles && \
        go run ./main.go
