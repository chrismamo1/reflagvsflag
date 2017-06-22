FROM golang:1.8
RUN go get github.com/gorilla/mux
RUN go get github.com/lib/pq
RUN apt-get update && apt-get install -y ruby
RUN gem install sass
RUN sass -v
RUN curl -sL https://deb.nodesource.com/setup_6.x | bash -
RUN apt-get install -y nodejs
