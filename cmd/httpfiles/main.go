package main

import (
	"log"
	"net/http"

	"crypto/md5"
	"encoding/json"
	"flag"

	"github.com/nameoffnv/httpfiles"
	"github.com/nameoffnv/httpfiles/middleware/limiter"
	"github.com/nameoffnv/httpfiles/storage"
	"github.com/nameoffnv/httpfiles/storage/fs"
	"github.com/nameoffnv/httpfiles/storage/redis_fs"
)

type Options struct {
	RedisHost     string
	RedisPassword string
	RedisDB       int
	StorePath     string
}

func main() {
	opts := Options{}

	flag.StringVar(&opts.RedisHost, "redis", "", "Redis host and port (ex. localhost:6379)")
	flag.StringVar(&opts.RedisPassword, "redispassword", "", "Redis password")
	flag.IntVar(&opts.RedisDB, "redisdb", 0, "Redis database")
	flag.StringVar(&opts.StorePath, "path", "./store", "Path to store files")
	flag.Parse()

	var s storage.Storage
	if opts.RedisHost != "" {
		redisStorage, err := redis_fs.New(opts.RedisHost, opts.RedisPassword, opts.RedisDB, opts.StorePath)
		if err != nil {
			log.Fatal(err)
		}
		s = redisStorage
	} else {
		s = fs.New(opts.StorePath, md5.New)
	}

	limit := limiter.New(limiter.Options{MaxRequestPerSecond: 1})
	filesMux, err := httpfiles.New(s)
	if err != nil {
		log.Fatal(err)
	}

	// stat func
	filesMux.Handle("/stat", filesMux.WithContext(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if opts.RedisHost == "" {
			rw.WriteHeader(http.StatusNotImplemented)
			return
		}

		ctxStorage := filesMux.GetStorage(req)
		if ctxStorage == nil {
			http.Error(rw, "storage not found", http.StatusInternalServerError)
			return
		}

		redisStorage, ok := ctxStorage.(*redis_fs.RedisFileStorage)
		if !ok {
			http.Error(rw, "storage cast to redis storage failed", http.StatusInternalServerError)
			return
		}

		stats, err := redisStorage.StatAll()
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(rw).Encode(stats)
	})))

	log.Printf("start listening :5000")
	if err := http.ListenAndServe(":5000", limit.LimitMiddleware(filesMux)); err != nil {
		log.Fatal(err)
	}
}
