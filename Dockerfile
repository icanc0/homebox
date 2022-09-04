# Build Nuxt
FROM node:17-alpine as frontend-builder
WORKDIR  /app
RUN npm install -g pnpm
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile --shamefully-hoist
COPY frontend .
RUN pnpm build



# Build API
FROM golang:alpine AS builder
RUN apk update
RUN apk upgrade
RUN apk add --update git build-base gcc g++
WORKDIR /go/src/app
COPY ./backend .
COPY --from=frontend-builder app/.output go/src/app/app/api/public
RUN go get -d -v ./...
RUN CGO_ENABLED=1 GOOS=linux go build -o /go/bin/api -v ./app/api/*.go


# Production Stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates
COPY ./backend/config.template.yml /app/config.yml
COPY --from=builder /go/bin/api /app

RUN chmod +x /app/api

LABEL Name=homebox Version=0.0.1
EXPOSE 7745
WORKDIR /app
CMD [ "./api" ]