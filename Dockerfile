#####
##### backend
#####
FROM golang:stretch as backend
RUN mkdir /var/www/ && chown -R www-data:www-data /var/www/

# add backend assets
ADD src/ /app/src
# chown by www-data
RUN chown -R www-data:www-data /app/
USER www-data:www-data

# and do the build
WORKDIR /app/src
RUN go build -o /app/hanabi-live

#####
##### frontend
#####
FROM node:stretch as frontend
RUN mkdir /var/www/ && chown -R www-data:www-data /var/www/

# add frontend assets
ADD .git /app/.git
ADD build_client.sh /app/build_client.sh
ADD public /app/public/

# chown everything by www-data and switch to the user
RUN chown -R www-data:www-data /app/
USER www-data:www-data

# install deps and build
WORKDIR /app/public/js
RUN npm install
RUN SENTRY_DSN="" ../../build_client.sh

#####
##### putting it all together
#####
FROM debian:stretch

RUN mkdir /var/www/ && chown -R www-data:www-data /var/www/
RUN apt-get update && apt-get install -y mysql-client && rm -rf /var/lib/apt/lists/*

# copy built assets
COPY --from=frontend /app/public/js/src/data/version.json /app//public/js/src/data/version.json
COPY --from=frontend /app/public/css/main.min.css /app/public/css/main.min.css
COPY --from=frontend /app/public/css/main.css /app/public/css/main.css
COPY --from=frontend /app/public/js/dist/ /app/public/js/dist/
COPY --from=backend /app/hanabi-live /app/hanabi-live

# add build assets
ADD install /app/install/
ADD public /app/public/
ADD docker/docker_entrypoint.sh /app/docker/docker_entrypoint.sh
ADD src /app/src/

# chown everything by www-data and switch to the user
RUN chown -R www-data:www-data /app/
USER www-data:www-data

# add environment variables
ENV HANABI_DOMAIN "localhost"
ENV HANABI_SESSION_SECRET "change_this_string"

ENV HANABI_DB_HOST "db"
ENV HANABI_DB_PORT "3306"
ENV HANABI_DB_NAME=""
ENV HANABI_DB_USER ""
ENV HANABI_DB_PASS ""

# use port 8080 and run the entry point
EXPOSE 8080

ENTRYPOINT [ "/app/docker/docker_entrypoint.sh" ]
CMD [ "/app/hanabi-live" ]