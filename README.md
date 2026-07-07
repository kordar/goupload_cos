# goupload_cos

腾讯云 COS 对象存储的 [goupload](https://github.com/kordar/goupload) 接口实现，适配 `BucketUploader` 全部操作。

## 安装

```bash
go get github.com/kordar/goupload_cos
```

## 快速开始

```go
import (
    "context"
    cosupload "github.com/kordar/goupload_cos"
)

// 创建 COS 客户端
uploader := cosupload.NewCOSClient("bucket-name", "ap-guangzhou", "secretId", "secretKey")

// 上传文件
err := uploader.PutFromFile(context.Background(), "path/to/obj", "./local-file.txt")

// 下载文件
data, err := uploader.Get(context.Background(), "path/to/obj")

// 列出目录
objects, next := uploader.List(context.Background(), "dir/", "", 100, false)

// 删除文件
err = uploader.Del(context.Background(), "path/to/obj")
```

## 实现的接口

`CosUploader` 实现了 `goupload.BucketUploader` 的全部方法：

| 分类 | 方法 |
|------|------|
| 元信息 | `Name()` / `Driver()` / `RemoteBuckets()` |
| 上传 | `Put()` / `PutString()` / `PutFromFile()` |
| 下载 | `Get()` / `GetToFile()` |
| 列表 | `List()` / `Count()` / `Tree()` |
| 删除 | `Del()` / `DelAll()` / `DelMulti()` |
| 管理 | `Copy()` / `Move()` / `Rename()` / `IsExist()` |
| 追加 | `Append()` / `AppendString()` |

## 依赖

- [goupload](https://github.com/kordar/goupload) — 上传接口定义
- [cos-go-sdk-v5](https://github.com/tencentyun/cos-go-sdk-v5) — 腾讯云 COS Go SDK

## License

MIT
