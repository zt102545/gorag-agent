# SQLite 迁移到 MySQL

仓库提供 `cmd/migrate` 一次性迁移命令。它会自动创建 MySQL 业务表，在事务中分批复制 SQLite 数据，并逐表输出源数据量、复制量和目标数据量。

## 迁移前准备

1. 停止 API Server 和 indexer，避免迁移过程中 SQLite 继续写入。
2. 备份 SQLite 文件。
3. 创建空的 MySQL 数据库，并确保连接账号拥有建表、读写和事务权限。
4. MySQL DSN 必须包含 `parseTime=true`，建议使用 `utf8mb4`：

```text
root:password@tcp(127.0.0.1:3306)/agent_cs?parseTime=true&charset=utf8mb4&loc=Local
```

## 执行迁移

PowerShell：

```powershell
$env:DATABASE_SQLITE_PATH = ".\data\app.db"
$env:DATABASE_MYSQL_DSN = "root:password@tcp(127.0.0.1:3306)/agent_cs?parseTime=true&charset=utf8mb4&loc=Local"
go run ./cmd/migrate
```

也可以直接传参：

```powershell
go run ./cmd/migrate -source ".\data\app.db" -mysql-dsn "root:password@tcp(127.0.0.1:3306)/agent_cs?parseTime=true&charset=utf8mb4&loc=Local" -batch-size 500
```

默认要求目标业务表为空，防止误覆盖。如果上一次已成功迁移，或确定需要按主键更新/合并已有数据，可显式执行：

```powershell
go run ./cmd/migrate -allow-existing
```

`-allow-existing` 会按主键执行 upsert，不会删除仅存在于 MySQL 的数据。

## 切换应用

确认命令输出 `migration completed` 且各表数量符合预期后，修改运行环境并启动服务：

```dotenv
DATABASE_DRIVER=mysql
DATABASE_MYSQL_DSN=root:password@tcp(127.0.0.1:3306)/agent_cs?parseTime=true&charset=utf8mb4&loc=Local
```

建议保留 SQLite 备份，完成业务抽查后再按数据保留策略归档。
