FROM golang:1.22

# Get environment variables
ARG DATABASE_URL
ARG REDIS_URL

# Install dependencies
WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/local/bin/main ./...
ENV DATABASE_URL=$DATABASE_URL
ENV REDIS_URL=$REDIS_URL
CMD ["main"]