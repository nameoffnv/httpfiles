package redis_fs

import (
	"fmt"
	"github.com/pkg/errors"
	"strconv"
	"time"
)

type FileMetaInfo struct {
	Filename      string
	Size          int64
	UploadDate    time.Time
	RemoveDate    *time.Time
	DownloadCount int
}

func (m FileMetaInfo) redisArgs() []interface{} {
	data := []interface{}{
		"filename", m.Filename,
		"size", fmt.Sprint(m.Size),
		"download_count", fmt.Sprint(m.DownloadCount),
		"upload_date", fmt.Sprint(m.UploadDate.Unix()),
	}

	if m.RemoveDate != nil {
		data = append(data, []interface{}{"remove_date", fmt.Sprint(m.RemoveDate.Unix())}...)
	}

	return data
}

func parseRedisMap(m map[string]string) (*FileMetaInfo, error) {
	meta := &FileMetaInfo{}
	for k, v := range m {
		switch k {
		case "size":
			size, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, errors.Wrap(err, "unmarshal")
			}
			meta.Size = size
		case "upload_date":
			ut, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, errors.Wrap(err, "unmarshal")
			}
			meta.UploadDate = time.Unix(ut, 0).UTC()
		case "remove_date":
			ut, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, errors.Wrap(err, "unmarshal")
			}
			t := time.Unix(ut, 0).UTC()
			meta.RemoveDate = &t
		case "download_count":
			count, err := strconv.Atoi(v)
			if err != nil {
				return nil, errors.Wrap(err, "unmarshal")
			}
			meta.DownloadCount = count
		case "filename":
			meta.Filename = v
		default:
			return nil, errors.Errorf("unknown filed %s", k)
		}
	}

	return meta, nil
}
