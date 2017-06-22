static/css/upload.css: views/css/upload.scss views/css/_reflagvsflag.scss
	sass views/css/upload.scss $@

static/css/judge.css: views/css/judge.scss views/css/_reflagvsflag.scss
	sass views/css/judge.scss $@

static/css/ranks.css: views/css/ranks.scss views/css/_reflagvsflag.scss
	sass views/css/ranks.scss $@

static/css/stats.css: views/css/stats.scss views/css/_reflagvsflag.scss
	sass views/css/stats.scss $@

views/react/dist/reflagvsflag.js:
	cd views/react && npm install --only=dev
	cd views/react && npm run bsb
	cd views/react && npm run dist

static/js/reflagvsflag.js: views/react/dist/reflagvsflag.js
	cp views/react/dist/reflagvsflag.js static/js/reflagvsflag.js

styles: static/css/judge.css static/css/ranks.css static/css/stats.css static/css/upload.css

run: main.go comparisonScheduler/comparisonScheduler.go things/things.go Dockerfile static/js/reflagvsflag.js
	bash -c docker run -it --rm --name reflagvsflag-instance -p 3456:3456 -v .:/go/src/reflagvsflag -w /go/src/github.com/chrismamo1/reflagvsflag reflagvsflag-image

reflagvsflag-image: main.go comparisonScheduler/comparisonScheduler.go things/things.go Dockerfile
	docker build -t reflagvsflag-image .
