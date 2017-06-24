FROM chrismamo1/reflagvsflag:reflagvsflag-base
EXPOSE 80
RUN go get github.com/chrismamo1/reflagvsflag
WORKDIR /go/src/github.com/chrismamo1
ADD . /reflagvsflag
WORKDIR /go/src/github.com/chrismamo1/reflagvsflag
RUN ls && pwd
RUN make styles
RUN eval `opam config env` && make static/js/reflagvsflag.js
CMD ls && pwd && \
        go run ./main.go
