# 企业微信会话内容存档 SDK

本项目嵌入企业微信官方 Linux x86 会话内容存档 C SDK。

- 官方文档：https://developer.work.weixin.qq.com/document/path/91774
- 官方下载：https://wwcdn.weixin.qq.com/node/wwcomm/sdk_x86_v3_20250205.tgz
- SDK 版本：`20250205`
- 官方包内 MD5：`f2db3dd1372c516db6290afbd1b5c698`
- 嵌入 `.so` SHA-256：`79ced4de6b18d5e96a21cd06f325794dc8957f8925120538d56d4ce827d3dfd0`

`WeWorkFinanceSdk_C.h`、`version.txt` 与 `md5.txt` 保留在本目录用于审计；
运行库位于 `internal/wecom/sdk/native/linux_amd64/`，由 `go:embed` 编入服务二进制。
