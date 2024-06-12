package goupload

import (
	"context"
	"fmt"
	logger "github.com/kordar/gologger"
	"github.com/kordar/goupload"
	"github.com/tencentyun/cos-go-sdk-v5"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type COSClient struct {
	bucket string
	region string
	client *cos.Client
}

func NewCOSClient(bucket string, region string, secretId string, secretKey string) *COSClient {
	bucketUrl, _ := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", bucket, region))
	serviceUrl, _ := url.Parse(fmt.Sprintf("https://cos.%s.myqcloud.com", region))
	b := &cos.BaseURL{BucketURL: bucketUrl, ServiceURL: serviceUrl}
	// 用于Get Service 查询，默认全地域 service.cos.myqcloud.com
	b.ServiceURL = serviceUrl

	// 1.永久密钥
	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  secretId,  // 替换为用户的 SecretId，请登录访问管理控制台进行查看和管理，https://console.cloud.tencent.com/cam/capi
			SecretKey: secretKey, // 替换为用户的 SecretKey，请登录访问管理控制台进行查看和管理，https://console.cloud.tencent.com/cam/capi
		},
	})

	return &COSClient{client: client, bucket: bucket, region: region}
}

// GetBucketName 获取bucket名称
func (c *COSClient) GetBucketName() string {
	return c.bucket
}

// CreateBucket 创建bucket
func (c *COSClient) CreateBucket() {
	_, err := c.client.Bucket.Put(context.Background(), nil)
	if err != nil {
		logger.Warnf("create bucket err = %v", err)
		return
	}
}

// Buckets bucket列表
func (c *COSClient) Buckets() []goupload.Bucket {
	s, _, err := c.client.Service.Get(context.Background())
	if err != nil {
		logger.Warnf("bucket err = %v", err)
		return make([]goupload.Bucket, 0)
	}

	buckets := make([]goupload.Bucket, 0)
	for _, b := range s.Buckets {
		bucket := goupload.Bucket{
			Name:       b.Name,
			Region:     b.Region,
			CreateTime: b.CreationDate,
		}
		buckets = append(buckets, bucket)
	}

	return buckets
}

// PutString 通过字符串上传对象，例如base64文件
func (c *COSClient) PutString(name string, content string) error {
	f := strings.NewReader(content)
	return c.Put(name, f)
}

// Put 上传字节流文件
func (c *COSClient) Put(name string, fd io.Reader) error {
	_, err := c.client.Object.Put(context.Background(), name, fd, nil)
	return err
}

// PutFromFile 上传本地文件
func (c *COSClient) PutFromFile(name string, filePath string) error {
	_, err := c.client.Object.PutFromFile(context.Background(), name, filePath, nil)
	return err
}

// List 获取文件列表
func (c *COSClient) List(path string, next string, limit int) goupload.BucketResult {
	opt := &cos.BucketGetOptions{
		Prefix:    path,
		Delimiter: "/",
		Marker:    next,
		MaxKeys:   limit,
	}

	v, _, err := c.client.Bucket.Get(context.Background(), opt)
	if err != nil {
		logger.Warnf("Get bucket list err = %v", err)
		return goupload.BucketResult{}
	}

	data := make([]goupload.Object, 0)
	for _, obj := range v.Contents {
		parse, _ := time.Parse(time.RFC3339, obj.LastModified)
		object := goupload.Object{
			Path:      obj.Key,
			Timestamp: parse.Unix(),
			Size:      obj.Size,
			Type:      obj.ETag,
			Width:     0,
			Height:    0,
		}
		data = append(data, object)
	}

	dirs := make([]string, 0)
	for _, commonPrefix := range v.CommonPrefixes {
		dirs = append(dirs, commonPrefix)
	}

	return goupload.BucketResult{Content: data, Dirs: dirs}
}

// Del 删除对象
func (c *COSClient) Del(name string) error {
	_, err := c.client.Object.Delete(context.Background(), name)
	return err
}

// Get 通过响应体获取对象
func (c *COSClient) Get(name string) ([]byte, error) {
	resp, err := c.client.Object.Get(context.Background(), name, nil)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(resp.Body)
}

// GetToFile 存储到本地文件
func (c *COSClient) GetToFile(name string, localPath string) error {
	_, err := c.client.Object.GetToFile(context.Background(), name, localPath, nil)
	return err
}
