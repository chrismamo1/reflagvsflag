FROM golang:1.8
RUN apt-get update
RUN apt-get install -y software-properties-common python-software-properties
RUN apt-get install -y m4 opam
RUN eval `opam config env --shell=sh`
RUN opam update
RUN opam switch 4.03.0
RUN opam install reason
RUN opam upgrade
RUN go get github.com/gorilla/mux
RUN go get github.com/lib/pq
RUN apt-get install -y ruby
RUN gem install sass
RUN sass -v
RUN curl -sL https://deb.nodesource.com/setup_6.x | bash -
RUN apt-get install -y nodejs
RUN ls
RUN pwd
