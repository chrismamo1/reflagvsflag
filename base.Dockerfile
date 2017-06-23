FROM golang:1.8
RUN apt-get update
RUN apt-get install -y software-properties-common python-software-properties m4 opam git ruby
RUN opam --help
RUN export OPAMYES=true
RUN echo $OPAMYES
RUN opam init
RUN eval `opam config env --shell=sh`
RUN opam switch 4.03.0
RUN eval `opam config env`
RUN opam update
RUN eval `opam config env`
RUN opam install reason
RUN opam upgrade
RUN go get github.com/gorilla/mux
RUN go get github.com/lib/pq
RUN gem install sass && sass -v
RUN curl -sL https://deb.nodesource.com/setup_6.x | bash -
RUN apt-get install -y nodejs
RUN ls && pwd
