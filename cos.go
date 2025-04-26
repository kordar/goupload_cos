package goupload_cos

import (
	"context"
	"fmt"
	"github.com/kordar/gologger"
	"github.com/kordar/goupload"
	"github.com/tencentyun/cos-go-sdk-v5"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
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
	b := &cos.BaseURL{BucketURL: bucketUrl, ServiceURL: serviceUrl}
	// 用于Get Service 查询，默认全地域 service.cos.myqcloud.com
	b.ServiceURL = serviceUrl

	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{SecretID: secretId, SecretKey: secretKey},
	})

	return &CosUploader{
		client:     client,
		bucketName: bucketName,
		region:     region,
	}
}

func (c *CosUploader) RemoteBuckets(ctx context.Context, args ...interface{}) []goupload.Bucket {
	s, _, err := c.client.Service.Get(ctx)
	if err != nil {
		logger.Warnf("[%s,%s] get remote bucket err = %v", c.Driver(), c.Name(), err)
		return make([]goupload.Bucket, 0)
	}

	buckets := make([]goupload.Bucket, 0)
	for _, b := range s.Buckets {
		bucket := goupload.Bucket{
			Name:   b.Name,
			Driver: c.Driver(),
			Params: make(map[string]interface{}),
		}
		buckets = append(buckets, bucket)
	}

	return buckets
}

func (c *CosUploader) Get(ctx context.Context, name string, args ...interface{}) ([]byte, error) {
	var opt *cos.ObjectGetOptions = nil
	if len(args) > 0 {
		opt = args[0].(*cos.ObjectGetOptions)
	}
	resp, err := c.client.Object.Get(ctx, name, opt)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(resp.Body)
}

func (c *CosUploader) GetToFile(ctx context.Context, name string, localPath string, args ...interface{}) error {
	if len(args) > 0 {
		opt := args[0].(*cos.ObjectGetOptions)
		_, err := c.client.Object.GetToFile(context.Background(), name, localPath, opt)
		return err
	} else {
		_, err := c.client.Object.GetToFile(context.Background(), name, localPath, nil)
		return err
	}
}

func (c *CosUploader) Put(ctx context.Context, name string, fd io.Reader, args ...interface{}) error {
	if len(args) > 0 {
		opt := args[0].(*cos.ObjectPutOptions)
		_, err := c.client.Object.Put(ctx, name, fd, opt)
		return err
	} else {
		_, err := c.client.Object.Put(ctx, name, fd, nil)
		return err
	}
}

// PutString 通过字符串上传对象，例如base64文件
func (c *CosUploader) PutString(ctx context.Context, name string, content string, args ...interface{}) error {
	f := strings.NewReader(content)
	return c.Put(ctx, name, f, args...)
}

func (c *CosUploader) PutFromFile(ctx context.Context, name string, filePath string, args ...interface{}) error {
	if len(args) > 0 {
		_, err := c.client.Object.PutFromFile(ctx, name, filePath, args[0].(*cos.ObjectPutOptions))
		return err
	} else {
		_, err := c.client.Object.PutFromFile(ctx, name, filePath, nil)
		return err
	}
}

func (c *CosUploader) List(ctx context.Context, dir string, next interface{}, limit int, subCount bool, args ...interface{}) ([]goupload.BucketObject, interface{}) {
	var marker = next.(string)
	data := make([]goupload.BucketObject, 0)
	opt := &cos.BucketGetOptions{
		Prefix:    dir,
		Delimiter: "/",
		MaxKeys:   limit,
	}
	total := 0
	isTruncated := true
	for isTruncated {
		opt.Marker = marker
		v, _, err := c.client.Bucket.Get(ctx, opt)
		if err != nil {
			logger.Warn(err)
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
				total++
				if total >= limit {
					return data, v.NextMarker
				}
			}
		}

		// common prefix 表示表示被 delimiter 截断的路径, 如 delimter 设置为/, common prefix 则表示所有子目录的路径
		for _, commonPrefix := range v.CommonPrefixes {
			data = append(data, goupload.BucketObject{
				Id:       commonPrefix,
				Path:     commonPrefix,
				FileType: "dir",
				Params: map[string]interface{}{
					"filename": path.Base(commonPrefix),
				},
			})
			total++
			if total >= limit {
				return data, v.NextMarker
			}
		}
		isTruncated = v.IsTruncated // 是否还有数据
		marker = v.NextMarker       // 设置下次请求的起始 key
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
			logger.Warn(err)
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

		isTruncated = v.IsTruncated // 是否还有数据
		marker = v.NextMarker       // 设置下次请求的起始 key
	}

	return total
}

func (c *CosUploader) Del(ctx context.Context, name string, args ...interface{}) error {
	if len(args) > 0 {
		_, err := c.client.Object.Delete(ctx, name, args[0].(*cos.ObjectDeleteOptions))
		return err
	} else {
		_, err := c.client.Object.Delete(ctx, name)
		return err
	}
}

func (c *CosUploader) DelAll(ctx context.Context, dir string, args ...interface{}) {
	var marker string
	opt := &cos.BucketGetOptions{
		Prefix:  dir,
		MaxKeys: 1000,
	}
	isTruncated := true
	for isTruncated {
		opt.Marker = marker
		v, _, err := c.client.Bucket.Get(ctx, opt)
		if err != nil {
			// Error
			break
		}
		for _, content := range v.Contents {
			_, err = c.client.Object.Delete(ctx, content.Key)
			if err != nil {
				// Error
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

	if obs != nil && len(obs) > 0 {
		opt := &cos.ObjectDeleteMultiOptions{
			Objects: obs,
			// 布尔值，这个值决定了是否启动 Quiet 模式
			// 值为 true 启动 Quiet 模式，值为 false 则启动 Verbose 模式，默认值为 false
			// Quiet: true,
		}
		_, _, err := c.client.Object.DeleteMulti(ctx, opt)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *CosUploader) IsExist(ctx context.Context, name string, args ...interface{}) (bool, error) {
	id := make([]string, 0)
	if len(args) > 0 {
		for _, arg := range args {
			id = append(id, arg.(string))
		}
	}
	return c.client.Object.IsExist(ctx, name, id...)
}

func (c *CosUploader) Copy(ctx context.Context, dest string, source string, args ...interface{}) error {
	baseUrl := fmt.Sprintf("%s.cos.%s.myqcloud.com", c.bucketName, c.region)
	sourceURL := path.Join(baseUrl, source)
	_, _, err := c.client.Object.Copy(ctx, dest, sourceURL, nil)
	return err
}

func (c *CosUploader) Move(ctx context.Context, dest string, source string, args ...interface{}) error {
	if err := c.Copy(ctx, dest, source, args...); err != nil {
		return err
	} else {
		_ = c.Del(ctx, source)
	}
	return nil
}

func (c *CosUploader) Rename(ctx context.Context, dest string, source string, args ...interface{}) error {
	return c.Move(ctx, dest, source, args...)
}

func (c *CosUploader) Tree(ctx context.Context, dir string, next interface{}, limit int, dep int, maxDep int, noleaf bool, subCount bool, args ...interface{}) []goupload.BucketTreeObject {
	if dep > maxDep {
		return []goupload.BucketTreeObject{}
	}

	var marker = next.(string)
	data := make([]goupload.BucketTreeObject, 0)
	opt := &cos.BucketGetOptions{Prefix: dir, Delimiter: "/", MaxKeys: limit}
	isTruncated := true
	for isTruncated {
		opt.Marker = marker
		v, _, err := c.client.Bucket.Get(ctx, opt)
		if err != nil {
			logger.Warn(err)
			break
		}

		if !noleaf {
			for _, content := range v.Contents {
				if content.Key == dir {
					continue
				}
				data = append(data, goupload.BucketTreeObject{
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
					Children: make([]goupload.BucketTreeObject, 0),
				})
			}
		}

		// common prefix 表示表示被 delimiter 截断的路径, 如 delimter 设置为/, common prefix 则表示所有子目录的路径
		for _, commonPrefix := range v.CommonPrefixes {
			count := 0
			if subCount {
				count = c.Count(ctx, commonPrefix)
			}
			children := c.Tree(ctx, commonPrefix, "", limit, dep+1, maxDep, noleaf, subCount, args...)
			data = append(data, goupload.BucketTreeObject{
				Id:       commonPrefix,
				Path:     commonPrefix,
				FileType: "dir",
				Params: map[string]interface{}{
					"count":    count,
					"filename": path.Base(commonPrefix),
				},
				Children: children,
			})
		}
		isTruncated = v.IsTruncated // 是否还有数据
		marker = v.NextMarker       // 设置下次请求的起始 key
	}

	return data
}

func (c *CosUploader) Append(ctx context.Context, name string, position int, fd io.Reader, args ...interface{}) (int, error) {
	if len(args) > 0 {
		opt := args[0].(*cos.ObjectPutOptions)
		pos, _, err := c.client.Object.Append(ctx, name, position, fd, opt)
		return pos, err
	} else {
		pos, _, err := c.client.Object.Append(ctx, name, position, fd, nil)
		return pos, err
	}
}

func (c *CosUploader) AppendString(ctx context.Context, name string, position int, content string, args ...interface{}) (int, error) {
	reader := strings.NewReader(content)
	return c.Append(ctx, name, position, reader, args...)
}
