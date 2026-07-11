# goupload_cos

腾讯云 COS 对象存储的 [goupload](https://github.com/kordar/goupload) 驱动实现：使用 `cos-go-sdk-v5` 适配 `goupload.BucketUploader`，并实现可选能力（原生分片上传、流式/范围读取）。

## 安装

```bash
go get github.com/kordar/goupload_cos
```

## 快速开始

```go
import (
    "context"
    "fmt"
    "os"
    cosupload "github.com/kordar/goupload_cos"
    "github.com/kordar/goupload"
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

// 可选能力：分片上传
if mp, ok := goupload.AsMultipartUploader(uploader); ok {
    uploadID, _ := mp.InitMultipart(context.Background(), "big.bin")
    f, _ := os.Open("./big.bin")
    defer f.Close()
    etag, _ := mp.UploadPart(context.Background(), "big.bin", uploadID, 1, f, 0)
    _ = mp.CompleteMultipart(context.Background(), "big.bin", uploadID, []goupload.CompletedPart{{PartNumber: 1, ETag: etag}})
}

// 可选能力：流式/范围读取
if s, ok := goupload.AsStreamer(uploader); ok {
    rc, total, err := goupload.OpenObject(context.Background(), uploader, "path/to/obj", &goupload.GetOptions{RangeStart: 0, RangeEnd: 1023})
    fmt.Println("total:", total, "err:", err)
    if rc != nil {
        defer rc.Close()
    }
}
```

## 与 goupload.Manager 配合

`CosUploader` 可直接作为 `goupload.BucketUploader` 注册到 `UploaderManager`（注意：Manager 内部使用 `context.Background()` 调用）：

```go
mgr := goupload.NewManagerWithUploader(uploader)
_ = mgr.PutFromFile(uploader.Name(), "dir/a.txt", "./a.txt")
```

## 实现的接口

`CosUploader` 实现了以下接口：

| 分类 | 方法 |
|------|------|
| 元信息 | `Name()` / `Driver()` / `RemoteBuckets()` |
| 上传 | `Put()` / `PutString()` / `PutFromFile()` |
| 下载 | `Get()` / `GetToFile()` |
| 列表 | `List()` / `Count()` / `Tree()` |
| 删除 | `Del()` / `DelAll()` / `DelMulti()` |
| 管理 | `Copy()` / `Move()` / `Rename()` / `IsExist()` |
| 追加 | `Append()` / `AppendString()` |
| 可选 | `BucketMultipartUploader`（原生分片上传） |
| 可选 | `BucketStreamer`（Stat/Open，流式与 Range 读取） |

## COS Options 透传（args 可变参）

部分方法的 `args ...interface{}` 用于透传 COS SDK 的 options：

- `Put` / `PutFromFile`：第 1 个参数可传 `*cos.ObjectPutOptions`
- `Get` / `GetToFile`：第 1 个参数可传 `*cos.ObjectGetOptions`
- `Del`：第 1 个参数可传 `*cos.ObjectDeleteOptions`

注意：当前实现使用强类型断言（例如 `args[0].(*cos.ObjectPutOptions)`），传错类型会 panic；`List/Tree` 的 `next` 也要求为 `string`（marker），传错同样会 panic。

## 依赖

- [goupload](https://github.com/kordar/goupload) — 上传接口定义
- [cos-go-sdk-v5](https://github.com/tencentyun/cos-go-sdk-v5) — 腾讯云 COS Go SDK

## License

README 当前声明 MIT，但仓库内未提供 LICENSE 文件，使用前请确认授权方式。
