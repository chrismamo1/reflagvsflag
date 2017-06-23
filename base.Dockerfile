FROM golang:1.8
RUN apt-get install software-properties-common python-software-properties
RUN add-apt-repository --yes ppa:avsm/ppa
RUN apt-get update -qq
RUN apt-get install -y m4 opam
RUN bash -c "eval $(opam config env)"
RUN bash -c "opam switch 4.03.0"
RUN go get github.com/gorilla/mux
RUN go get github.com/lib/pq
RUN apt-get update && apt-get install -y ruby
RUN gem install sass
RUN sass -v
RUN curl -sL https://deb.nodesource.com/setup_6.x | bash -
RUN apt-get install -y nodejs
RUN ls
RUN pwd
