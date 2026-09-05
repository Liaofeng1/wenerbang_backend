# 问而帮 · Backend

校园问卷互助平台服务端。问卷本身在外部链接（如问卷星）填写，本服务负责：账号与画像、积分/经验、发卷扣费、大厅分发、填卷会话与结算、悬赏/置顶/分类投放、下架、以及基于停留时长的举报治理。

**技术栈：** Go 1.22 · Gin · GORM · SQLite · JWT  

**配套前端：** [wenerbang_frontend](https://github.com/Liaofeng1/wenerbang_frontend)

---

## 功能概览

### 账号与画像
- 注册 / 登录（JWT）
- 注册赠送积分；邀请码注册双方再获奖励
- 画像：学校、**学科门类（必填）**、性别、南北方、城市线级
- 资料可在 `/me` 更新

### 积分与经验（分开计算）
| 类型 | 用途 |
|------|------|
| **积分** | 发卷、置顶、分类投放等消费 |
| **经验** | 等级（Lv.1–7）、称号、置顶/投放折扣、每月免费置顶次数 |

| 行为 | 积分 | 经验 |
|------|------|------|
| 注册 | +30（可配置） | — |
| 邀请成功（双方） | 各 +50（可配置） | — |
| 每日签到 | +10 | +5 |
| 有效填卷 | 按时长公式 + 可选悬赏 | +10 |
| 成功发卷 | 扣除发卷费用 | +30 |

等级经验阈值（累计）：`0 / 15 / 120 / 280 / 480 / 720 / 1000` → Lv.1–7。

### 发卷费用（对齐方案 §3.2.2）
1. **基础发布费 150**：沉没成本，与最终回收份数无关  
2. **额外激励（悬赏）**：可选；至少前 50 份、每份至少 10 分；发布时冻结奖池；**奖池未用完期间自动置顶**，用完取消  
3. **付费置顶**：约 30 分/小时，档位 4 / 6 / 8 小时（可与悬赏置顶叠加；高等级有折扣或免费次数）  
4. **分类投放**：按学校 / 学科门类 / 性别；费用 = 要求人数 × 5；仅画像匹配用户可见；触达上限约要求人数 × 200%  

另需填写 **下架天数（1–60）**，到期自动关闭并退回剩余冻结悬赏（若有）。

### 填卷与结算
- 流程：`start` → `leave`（打开外链）→ `return` → `complete`
- 停留低于发布者设定的 **Tmin** 不能领分
- 平台填卷奖励按当前平均时长的高斯型曲线结算；悬赏名额内额外从发布者奖池转移

### 治理
- 发布者可对填写者 **举报**（须已有完成记录）
- 系统用参考平均时长判断过快 / 过慢；正常则举报无效
- 有效警告累计，默认满 **3** 次封禁 **14** 天；封禁期间不可发卷、不可填卷
- 到期自动解封并清零警告计数

> 说明：问卷答卷在外部平台，本服务的举报针对的是**平台账号与停留行为**，无法自动对应外部表单中的某一份答卷内容。

---

## 环境要求

- Go **1.22+**
- 无需单独安装数据库（默认 SQLite 文件）

---

## 快速启动

```bash
# 进入本仓库根目录（含 go.mod、cmd/）
go mod tidy
go run ./cmd/server
# 或：go build -o wenbang-server ./cmd/server && ./wenbang-server
```

- 默认监听：`http://127.0.0.1:8080`
- 健康检查：`GET /healthz` → `{"status":"ok"}`
- 数据库文件：当前工作目录下 `wenbang.db`（可用环境变量改路径）
- 表结构启动时 **AutoMigrate**，首次运行自动建表

可选：复制 `.env.example` 为环境变量来源（当前进程需自行 export / 使用你的加载方式；代码通过 `os.Getenv` 读取）。

**清空数据：** 停止进程后删除 `wenbang.db`，再启动即可。

---

## 主要 API（前缀 `/api/v1`）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/auth/register` | 注册 |
| POST | `/auth/login` | 登录 |
| GET | `/meta/profile-options` | 画像枚举 |
| GET/PATCH/PUT | `/me` | 当前用户 / 更新资料 |
| POST | `/me/checkin` | 每日签到 |
| POST | `/surveys` | 发布问卷 |
| GET | `/surveys` | 大厅列表（含定向过滤、置顶排序） |
| GET | `/surveys/mine` | 我发布的 |
| GET | `/surveys/:id` | 详情 |
| GET | `/surveys/:id/stats` | 发布者统计 |
| POST | `/surveys/:id/close` | 手动关闭 |
| POST | `/surveys/:id/start\|leave\|return\|complete` | 填卷会话 |
| POST | `/surveys/:id/report` | 举报填写者 `{ "user_id": number }` |
| GET | `/completions/mine` | 我的填写记录 |

除注册、登录、profile-options、healthz 外，均需请求头：

```http
Authorization: Bearer <token>
```

---

## 环境变量

| 变量 | 默认 | 含义 |
|------|------|------|
| `PORT` | `8080` | 监听端口 |
| `APP_ENV` | `dev` | 环境标记 |
| `JWT_SECRET` | `dev-secret-change-in-production` | JWT 密钥（**生产务必修改**） |
| `DATABASE_DSN` | `file:wenbang.db?cache=shared&mode=rwc` | SQLite DSN |
| `REGISTER_BONUS` | `30` | 注册赠分 |
| `INVITE_REWARD` | `50` | 邀请双方各得 |
| `PUBLISH_COST` | `150` | 基础发卷费 |
| `MIN_AWAY_SECONDS` | `120` | （保留配置；实际 Tmin 以问卷字段为准） |
| `PIN_HOURLY_COST` | `30` | 置顶单价（分/小时） |
| `BOUNTY_MIN_COUNT` | `50` | 悬赏最少份数 |
| `BOUNTY_MIN_PER` | `10` | 悬赏每份最少积分 |
| `TARGETING_COST_PER_USER` | `5` | 分类投放单价 |
| `TARGETING_DELIVERY_MULT` | `2` | 投放触达倍数（相对要求人数） |
| `MAX_SHELF_DAYS` | `60` | 最长下架天数 |
| `REPORT_FAST_RATIO` | `0.5` | 举报：过快阈值（相对参考平均） |
| `REPORT_SLOW_RATIO` | `2.0` | 举报：过慢阈值 |
| `WARN_LIMIT` | `3` | 封禁前警告次数 |
| `BAN_DAYS` | `14` | 封禁天数 |

---

## 目录结构

```
cmd/server/          入口
internal/
  app/               启动、迁移
  auth/              JWT
  config/            环境变量
  http/              handler / middleware / router
  level/             等级与经验
  model/             数据模型
  points/            填卷奖励公式
  service/           业务（auth / survey / moderation）
```

---

## 与前端联调

1. 先启动本服务（8080）  
2. 再启动前端开发服（默认 5173）；Vite 已将 `/api`、`/healthz` 代理到本服务  
3. 浏览器只访问前端地址即可  

生产部署时请更换 `JWT_SECRET`、收紧 CORS，并为 SQLite 选择持久磁盘路径（或迁移至 PostgreSQL）。

---

## License / 说明

黑客松演示项目。业务规则以产品文档与当前代码为准。
