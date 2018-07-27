package redis_fs

import (
	"strings"
	"testing"
	"time"
)

func TestFileMetaInfo(t *testing.T) {
	meta := FileMetaInfo{
		Filename:      "test",
		Size:          110,
		UploadDate:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		DownloadCount: 0,
	}
	removeDate := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	meta.RemoveDate = &removeDate

	t.Run("marshal", func(t *testing.T) {
		rargs := meta.redisArgs()
		args := make([]string, len(rargs))
		for i, v := range rargs {
			args[i] = v.(string)
		}

		if strings.Join(args, " ") != "filename test size 110 download_count 0 upload_date 1577836800 remove_date 1609459200" {
			t.Fatalf("marshal failed, actual '%s'", meta.redisArgs())
		}
	})

	t.Run("unmarshal", func(t *testing.T) {
		rMap := map[string]string{
			"filename":       "test",
			"size":           "110",
			"download_count": "0",
			"upload_date":    "1577836800",
			"remove_date":    "1609459200",
		}

		newMeta, err := parseRedisMap(rMap)
		if err != nil {
			t.Fatalf("unmarshal failed, error %v", err)
		}

		if newMeta.Filename != meta.Filename {
			t.Fatalf("filename mismatch excepted %s actual %s", meta.Filename, newMeta.Filename)
		}

		if newMeta.Size != meta.Size {
			t.Fatalf("size mismatch excepted %d actual %d", meta.Size, newMeta.Size)
		}

		if newMeta.UploadDate != meta.UploadDate {
			t.Fatalf("upload date mismatch excepted %d actual %d", meta.UploadDate.Unix(), newMeta.UploadDate.Unix())
		}

		if *newMeta.RemoveDate != *meta.RemoveDate {
			t.Fatalf("remove date mismatch excepted %d actual %d", meta.RemoveDate.Unix(), newMeta.RemoveDate.Unix())
		}

		if newMeta.DownloadCount != meta.DownloadCount {
			t.Fatalf("download count mismatch excepted %d actual %d", meta.DownloadCount, newMeta.DownloadCount)
		}
	})
}
