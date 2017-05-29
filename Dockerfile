FROM chrismamo1/reflagvsflag:base
RUN pwd && ls && go run ./main.go
