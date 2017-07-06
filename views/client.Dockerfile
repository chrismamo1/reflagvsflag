FROM chrismamo1/reflagvsflag:alpine-clientbase
ARG AWS_ACCESS_KEY_ID
ARG AWS_SECRET_ACCESS_KEY
COPY . /home/views
WORKDIR /home/views/react
RUN printenv && \
        eval `opam config env` && \
        npm run bsb-clean && \
        npm run bsb-world && \
        npm run bsb && \
        npm run dist && \
        aws s3 cp ./dist/reflagvsflag.js s3://reflagvsflag-static-files/scripts/reflagvsflag.js
WORKDIR /home/views/css
RUN pwd
RUN sass judge.scss judge.css && \
        sass ranks.scss ranks.css && \
        sass stats.scss stats.css && \
        sass upload.scss upload.css && \
        sass _reflagvsflag.scss reflagvsflag.css
RUN ls
RUN for file in ./*.scss; do aws s3 cp $file s3://reflagvsflag-static-files/styles/; done
RUN for file in ./*.css; do aws s3 cp $file s3://reflagvsflag-static-files/styles/; done
RUN for file in ./*.css.map; do aws s3 cp $file s3://reflagvsflag-static-files/styles/; done
