# 问而帮 Backend

Go + Gin + GORM + SQLite + JWT。

## 启动

```bash
cd backend
go mod tidy
go run ./cmd/server
```

默认监听 `http://127.0.0.1:8080`，数据库文件 `wenbang.db`。

环境变量见 `.env.example`（也可直接用默认值，无需 `.env`）。
