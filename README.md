# Outyet README

Build Status: [![Build Status](http://127.0.0.1:10081/buildStatus/icon?job=outyet)](http://127.0.0.1:10081/job/outyet/)

Forked from [golang/example](https://github.com/golang/example)

## Build

```sh
docker buildx build --tag outyet:latest . --load
```

### Run

```sh
docker run -e envvar=test -e secret=123 -p 8080:8080 outyet:latest
```
