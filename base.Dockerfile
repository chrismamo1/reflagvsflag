FROM golang:1.8
ENV OPAMYES true
RUN apt-get update && \
      apt-get install -y m4 opam ruby && \
      apt-get autoremove && \
      apt-get clean
RUN opam init && \
      eval `opam config env` && \
      opam switch 4.03.0 && \
      eval `opam config env` && \
      opam update && \
      opam upgrade && \
      opam install reason \
RUN go get github.com/gorilla/mux && \
      go get github.com/lib/pq
RUN gem install sass && \
      sass -v
RUN curl -sL https://deb.nodesource.com/setup_6.x | bash -
RUN apt-get install -y nodejs && npm update && ls && pwd
