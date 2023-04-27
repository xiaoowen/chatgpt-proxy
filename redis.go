package main

import "github.com/go-redis/redis/v8"

var rd *redis.Client

func initRedis() {
	rd = redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
}
