package goupload_cos

import (
	"context"
	"io"
	"sort"
	"strings"

	"github.com/kordar/goupload"
	"github.com/tencentyun/cos-go-sdk-v5"
)

func (c *CosUploader) InitMultipart(ctx context.Context, name string) (string, error) {
	result, _, err := c.client.Object.InitiateMultipartUpload(ctx, name, nil)
	if err != nil {
		return "", err
	}
	return result.UploadID, nil
}

func (c *CosUploader) UploadPart(ctx context.Context, name, uploadID string, partNumber int, r io.Reader, contentLength int64) (string, error) {
	opt := &cos.ObjectUploadPartOptions{}
	if contentLength > 0 {
		opt.ContentLength = contentLength
	}
	resp, err := c.client.Object.UploadPart(ctx, name, uploadID, partNumber, r, opt)
	if err != nil {
		return "", err
	}
	etag := strings.Trim(resp.Header.Get("ETag"), "\"")
	return etag, nil
}

func (c *CosUploader) CompleteMultipart(ctx context.Context, name, uploadID string, parts []goupload.CompletedPart) error {
	cosParts := make([]cos.Object, len(parts))
	for i, part := range parts {
		cosParts[i] = cos.Object{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		}
	}
	sort.Sort(cos.ObjectList(cosParts))
	_, _, err := c.client.Object.CompleteMultipartUpload(ctx, name, uploadID, &cos.CompleteMultipartUploadOptions{
		Parts: cosParts,
	})
	return err
}

func (c *CosUploader) AbortMultipart(ctx context.Context, name, uploadID string) error {
	_, err := c.client.Object.AbortMultipartUpload(ctx, name, uploadID)
	return err
}

func (c *CosUploader) ListParts(ctx context.Context, name, uploadID string) ([]goupload.MultipartPart, error) {
	result, _, err := c.client.Object.ListParts(ctx, name, uploadID, nil)
	if err != nil {
		return nil, err
	}
	parts := make([]goupload.MultipartPart, 0, len(result.Parts))
	for _, part := range result.Parts {
		parts = append(parts, goupload.MultipartPart{
			PartNumber: part.PartNumber,
			ETag:       strings.Trim(part.ETag, "\""),
			Size:       part.Size,
		})
	}
	return parts, nil
}

var _ goupload.BucketMultipartUploader = (*CosUploader)(nil)
