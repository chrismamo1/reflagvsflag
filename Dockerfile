FROM chrismamo1/reflagvsflag:reflagvsflag-base
EXPOSE 80
RUN go get github.com/chrismamo1/reflagvsflag
WORKDIR /go/src/github.com/chrismamo1
ADD . /reflagvsflag

WORKDIR /go/src/github.com/chrismamo1/reflagvsflag
CMD ls && pwd && go run ./main.go
