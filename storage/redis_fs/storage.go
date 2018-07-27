package redis_fs

import (
	"fmt"
	"github.com/go-redis/redis"
	"github.com/nameoffnv/httpfiles/storage"
	"github.com/nameoffnv/httpfiles/storage/fs"
	"github.com/pkg/errors"
	"io"
	"time"
)

const (
	keyLoadedFiles = "files"
	keyFileInfo    = "meta"
)

type RedisFileStorage struct {
	fs     *fs.FileStorage
	client *redis.Client
}

func New(redisHost, redisPassword string, redisDB int, path string) (storage.Storage, error) {
	fileStorage := fs.New(path)

	client := redis.NewClient(&redis.Options{
		Addr:     redisHost,
		Password: redisPassword,
		DB:       redisDB,
	})

	if _, err := client.Ping().Result(); err != nil {
		return nil, errors.Wrap(err, "redis ping")
	}

	return &RedisFileStorage{fileStorage.(*fs.FileStorage), client}, nil
}

func (s *RedisFileStorage) Get(id string) (io.ReadCloser, error) {
	if _, err := s.client.HExists(keyLoadedFiles, id).Result(); err == redis.Nil {
		return nil, storage.ErrNotFound
	} else if err != nil {
		return nil, errors.Wrap(err, "redis HExists")
	}

	reader, err := s.fs.Get(id)
	if err != nil {
		return nil, err
	}

	if _, err := s.client.HIncrBy(metaKey(id), "download_count", 1).Result(); err != nil {
		return nil, errors.Wrap(err, "redis HIncrBy")
	}

	return reader, nil
}

func (s *RedisFileStorage) Save(id string, data io.Reader) (int64, error) {
	n, err := s.fs.Save(id, data)
	if err != nil {
		return 0, errors.Wrap(err, "save file")
	}

	if _, err := s.client.HSet(keyLoadedFiles, id, true).Result(); err != nil {
		return 0, errors.Wrap(err, "redis HSet")
	}

	metaInfo := FileMetaInfo{
		Filename:      id,
		Size:          n,
		UploadDate:    time.Now(),
		DownloadCount: 0,
	}

	args := []interface{}{"hmset", metaKey(id)}
	args = append(args, metaInfo.redisArgs()...)
	cmd := redis.NewStatusCmd(args...)

	s.client.Process(cmd)

	if _, err := cmd.Result(); err != nil {
		return 0, errors.Wrap(err, "redis HMSet")
	}

	return n, nil
}

func (s *RedisFileStorage) Delete(id string) error {
	if _, err := s.client.HExists(keyLoadedFiles, id).Result(); err == redis.Nil {
		return storage.ErrNotFound
	} else if err != nil {
		return errors.Wrap(err, "redis HExists")
	}

	if err := s.fs.Delete(id); err != nil {
		return errors.Wrap(err, "delete file")
	}

	if _, err := s.client.HDel("files", id).Result(); err != nil {
		return errors.Wrap(err, "redis HDel")
	}

	if _, err := s.client.HSet(metaKey(id), "remove_date", time.Now().Unix()).Result(); err != nil {
		return errors.Wrap(err, "redis HSet")
	}

	return nil
}

func (s *RedisFileStorage) StatAll() ([]FileMetaInfo, error) {
	files, err := s.client.HGetAll(keyLoadedFiles).Result()
	if err != nil {
		return nil, errors.Wrap(err, "redis HGetAll")
	}

	infoList := make([]FileMetaInfo, len(files))
	i := 0
	for k := range files {
		metaMap, err := s.client.HGetAll(metaKey(k)).Result()
		if err != nil {
			return nil, errors.Wrap(err, "redis HGetAll meta")
		}

		meta, err := parseRedisMap(metaMap)
		if err != nil {
			return nil, errors.Wrap(err, "parse redis map")
		}

		infoList[i] = *meta
		i++
	}

	return infoList, nil
}

func metaKey(id string) string {
	return fmt.Sprint(keyFileInfo, ".", id)
}
