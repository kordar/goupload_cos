package goupload_cos

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/kordar/goupload"
	"github.com/tencentyun/cos-go-sdk-v5"
)

type CosUploader struct {
	bucketName string
	region     string
	client     *cos.Client
}

func (c *CosUploader) Name() string {
	return c.bucketName
}

func (c *CosUploader) Driver() string {
	return "cos"
}

func NewCOSClient(bucketName string, region string, secretId string, secretKey string) *CosUploader {
	bucketUrl, _ := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", bucketName, region))
	serviceUrl, _ := url.Parse(fmt.Sprintf("https://cos.%s.myqcloud.com", region))

	client := cos.NewClient(&cos.BaseURL{BucketURL: bucketUrl, ServiceURL: serviceUrl}, &http.Client{
		Transport: &cos.AuthorizationTransport{SecretID: secretId, SecretKey: secretKey},
	})

	return &CosUploader{client: client, bucketName: bucketName, region: region}
}

// getOpt extracts the first argument as *T, returns nil if no args.
func getOpt[T any](args ...interface{}) *T {
	if len(args) > 0 {
		return args[0].(*T)
	}
	return nil
}

func (c *CosUploader) RemoteBuckets(ctx context.Context, args ...interface{}) []goupload.Bucket {
	s, _, err := c.client.Service.Get(ctx)
	if err != nil {
		slog.Warn("get remote bucket err", "driver", c.Driver(), "name", c.Name(), "err", err)
		return nil
	}

	buckets := make([]goupload.Bucket, 0)
	for _, b := range s.Buckets {
		buckets = append(buckets, goupload.Bucket{
			Name:   b.Name,
			Driver: c.Driver(),
			Params: make(map[string]interface{}),
		})
	}

	return buckets
}

func (c *CosUploader) Get(ctx context.Context, name string, args ...interface{}) ([]byte, error) {
	opt := getOpt[cos.ObjectGetOptions](args...)
	resp, err := c.client.Object.Get(ctx, name, opt)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *CosUploader) GetToFile(ctx context.Context, name string, localPath string, args ...interface{}) error {
	opt := getOpt[cos.ObjectGetOptions](args...)
	_, err := c.client.Object.GetToFile(ctx, name, localPath, opt)
	return err
}

func (c *CosUploader) Put(ctx context.Context, name string, fd io.Reader, args ...interface{}) error {
	opt := getOpt[cos.ObjectPutOptions](args...)
	_, err := c.client.Object.Put(ctx, name, fd, opt)
	return err
}

// PutString 通过字符串上传对象，例如base64文件
func (c *CosUploader) PutString(ctx context.Context, name string, content string, args ...interface{}) error {
	return c.Put(ctx, name, strings.NewReader(content), args...)
}

func (c *CosUploader) PutFromFile(ctx context.Context, name string, filePath string, args ...interface{}) error {
	opt := getOpt[cos.ObjectPutOptions](args...)
	_, err := c.client.Object.PutFromFile(ctx, name, filePath, opt)
	return err
}

func (c *CosUploader) List(ctx context.Context, dir string, next interface{}, limit int, subCount bool, args ...interface{}) ([]goupload.BucketObject, interface{}) {
	marker := next.(string)
	data := make([]goupload.BucketObject, 0)
	opt := &cos.BucketGetOptions{Prefix: dir, Delimiter: "/", MaxKeys: limit}

	isTruncated := true
	for isTruncated {
		opt.Marker = marker
		v, _, err := c.client.Bucket.Get(ctx, opt)
		if err != nil {
			slog.Warn("list bucket failed", "err", err)
			break
		}

		if !subCount {
			for _, content := range v.Contents {
				data = append(data, goupload.BucketObject{
					Id:           content.Key,
					Path:         content.Key,
					LastModified: content.LastModified,
					Size:         content.Size,
					FileType:     "file",
					FileExt:      path.Ext(content.Key),
					Params: map[string]interface{}{
						"owner":         content.Owner,
						"restoreStatus": content.RestoreStatus,
						"versionId":     content.VersionId,
						"storageTier":   content.StorageTier,
						"storageClass":  content.StorageClass,
						"partNumber":    content.PartNumber,
						"etag":          content.ETag,
						"filename":      path.Base(content.Key),
					},
				})
				if len(data) >= limit {
					return data, v.NextMarker
				}
			}
		}

		for _, commonPrefix := range v.CommonPrefixes {
			data = append(data, goupload.BucketObject{
				Id:       commonPrefix,
				Path:     commonPrefix,
				FileType: "dir",
				Params: map[string]interface{}{
					"filename": path.Base(commonPrefix),
				},
			})
			if len(data) >= limit {
				return data, v.NextMarker
			}
		}

		isTruncated = v.IsTruncated
		marker = v.NextMarker
	}

	return data, marker
}

func (c *CosUploader) Count(ctx context.Context, dir string, args ...interface{}) int {
	justFile := false
	if len(args) > 0 {
		justFile = args[0].(bool)
	}

	var marker string
	opt := &cos.BucketGetOptions{
		Prefix:    strings.Trim(dir, "/") + "/",
		Delimiter: "/",
		MaxKeys:   1000,
	}

	isTruncated := true
	total := 0
	for isTruncated {
		opt.Marker = marker
		v, _, err := c.client.Bucket.Get(ctx, opt)
		if err != nil {
			slog.Warn("count bucket failed", "err", err)
			break
		}

		for _, content := range v.Contents {
			if content.Key == dir {
				continue
			}
			total++
		}
		if !justFile {
			total += len(v.CommonPrefixes)
		}

		isTruncated = v.IsTruncated
		marker = v.NextMarker
	}

	return total
}

func (c *CosUploader) Del(ctx context.Context, name string, args ...interface{}) error {
	opt := getOpt[cos.ObjectDeleteOptions](args...)
	_, err := c.client.Object.Delete(ctx, name, opt)
	return err
}

func (c *CosUploader) DelAll(ctx context.Context, dir string, args ...interface{}) {
	var marker string
	opt := &cos.BucketGetOptions{Prefix: dir, MaxKeys: 1000}

	isTruncated := true
	for isTruncated {
		opt.Marker = marker
		v, _, err := c.client.Bucket.Get(ctx, opt)
		if err != nil {
			slog.Warn("delAll list failed", "err", err, "dir", dir)
			break
		}

		for _, content := range v.Contents {
			if _, err := c.client.Object.Delete(ctx, content.Key); err != nil {
				slog.Warn("delAll delete failed", "err", err, "key", content.Key)
			}
		}
		for _, prefix := range v.CommonPrefixes {
			c.DelAll(ctx, prefix, args...)
		}

		isTruncated = v.IsTruncated
		marker = v.NextMarker
	}
}

func (c *CosUploader) DelMulti(ctx context.Context, objects []goupload.BucketObject, args ...interface{}) error {
	var obs []cos.Object
	for _, v := range objects {
		if v.FileType == "file" {
			obs = append(obs, cos.Object{Key: v.Path})
		} else if v.FileType == "dir" {
			c.DelAll(ctx, v.Path, args...)
		}
	}

	if len(obs) > 0 {
		opt := &cos.ObjectDeleteMultiOptions{Objects: obs}
		_, _, err := c.client.Object.DeleteMulti(ctx, opt)
		return err
	}

	return nil
}

func (c *CosUploader) IsExist(ctx context.Context, name string, args ...interface{}) (bool, error) {
	var ids []string
	for _, arg := range args {
		ids = append(ids, arg.(string))
	}
	return c.client.Object.IsExist(ctx, name, ids...)
}

func (c *CosUploader) Copy(ctx context.Context, dest string, source string, args ...interface{}) error {
	sourceURL := fmt.Sprintf("%s.cos.%s.myqcloud.com", c.bucketName, c.region)
	_, _, err := c.client.Object.Copy(ctx, dest, path.Join(sourceURL, source), nil)
	return err
}

func (c *CosUploader) Move(ctx context.Context, dest string, source string, args ...interface{}) error {
	if err := c.Copy(ctx, dest, source, args...); err != nil {
		return err
	}
	return c.Del(ctx, source)
}

func (c *CosUploader) Rename(ctx context.Context, dest string, source string, args ...interface{}) error {
	return c.Move(ctx, dest, source, args...)
}

func (c *CosUploader) Tree(ctx context.Context, dir string, next interface{}, limit int, dep int, maxDep int, noleaf bool, subCount bool, args ...interface{}) []goupload.BucketTreeObject {
	if dep > maxDep {
		return nil
	}

	marker := next.(string)
	data := make([]goupload.BucketTreeObject, 0)
	opt := &cos.BucketGetOptions{Prefix: dir, Delimiter: "/", MaxKeys: limit}

	isTruncated := true
	for isTruncated {
		opt.Marker = marker
		v, _, err := c.client.Bucket.Get(ctx, opt)
		if err != nil {
			slog.Warn("tree list failed", "err", err, "dir", dir)
			break
		}

		if !noleaf {
			for _, content := range v.Contents {
				if content.Key == dir {
					continue
				}
				data = append(data, goupload.BucketTreeObject{
					BucketObject: goupload.BucketObject{
						Id:           content.Key,
						Path:         content.Key,
						LastModified: content.LastModified,
						Size:         content.Size,
						FileType:     "file",
						FileExt:      path.Ext(content.Key),
						Params: map[string]interface{}{
							"owner":         content.Owner,
							"restoreStatus": content.RestoreStatus,
							"versionId":     content.VersionId,
							"storageTier":   content.StorageTier,
							"storageClass":  content.StorageClass,
							"partNumber":    content.PartNumber,
							"etag":          content.ETag,
							"filename":      path.Base(content.Key),
						},
					},
					Children: nil,
				})
			}
		}

		for _, commonPrefix := range v.CommonPrefixes {
			count := 0
			if subCount {
				count = c.Count(ctx, commonPrefix)
			}
			children := c.Tree(ctx, commonPrefix, "", limit, dep+1, maxDep, noleaf, subCount, args...)
			data = append(data, goupload.BucketTreeObject{
				BucketObject: goupload.BucketObject{
					Id:       commonPrefix,
					Path:     commonPrefix,
					FileType: "dir",
					Params: map[string]interface{}{
						"count":    count,
						"filename": path.Base(commonPrefix),
					},
				},
				Children: children,
			})
		}

		isTruncated = v.IsTruncated
		marker = v.NextMarker
	}

	return data
}

func (c *CosUploader) Append(ctx context.Context, name string, position int, fd io.Reader, args ...interface{}) (int, error) {
	opt := getOpt[cos.ObjectPutOptions](args...)
	pos, _, err := c.client.Object.Append(ctx, name, position, fd, opt)
	return pos, err
}

func (c *CosUploader) AppendString(ctx context.Context, name string, position int, content string, args ...interface{}) (int, error) {
	return c.Append(ctx, name, position, strings.NewReader(content), args...)
}

func (c *CosUploader) Stat(ctx context.Context, name string) (int64, time.Time, error) {
	resp, err := c.client.Object.Head(ctx, name, nil)
	if err != nil {
		return 0, time.Time{}, err
	}
	modTime := time.Time{}
	if raw := resp.Header.Get("Last-Modified"); raw != "" {
		if parsed, parseErr := time.Parse(time.RFC1123, raw); parseErr == nil {
			modTime = parsed
		}
	}
	return resp.ContentLength, modTime, nil
}

func (c *CosUploader) Open(ctx context.Context, name string, opts *goupload.GetOptions) (io.ReadCloser, error) {
	var opt *cos.ObjectGetOptions
	if opts != nil {
		size, _, err := c.Stat(ctx, name)
		if err != nil {
			return nil, err
		}
		start, end, err := goupload.NormalizeRange(opts, size)
		if err != nil {
			return nil, err
		}
		if start > 0 || end < size-1 {
			opt = &cos.ObjectGetOptions{
				Range: fmt.Sprintf("bytes=%d-%d", start, end),
			}
		}
	}
	resp, err := c.client.Object.Get(ctx, name, opt)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
