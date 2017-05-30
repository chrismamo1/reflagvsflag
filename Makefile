run: main.go comparisonScheduler/comparisonScheduler.go things/things.go Dockerfile
	bash -c docker run -it --rm --name reflagvsflag-instance -p 3456:3456 -v .:/go/src/reflagvsflag -w /go/src/github.com/chrismamo1/reflagvsflag reflagvsflag-image

reflagvsflag-image: main.go comparisonScheduler/comparisonScheduler.go things/things.go Dockerfile
	docker build -t reflagvsflag-image .
