# 🌱 谷香认养 · 农产品众筹认养配送与成熟度回访平台

围绕"同一认养批次"把 **农场、认养人、采摘工、包装员、物流、客服** 的全部处理结果串成一条链：
地块 → 农事投入 → 施肥用药 → 天气灾害 → 成熟度抽检 → **成熟度分批发货（成熟不均拆多轮，逐轮预计重量+口感差异）** →
采摘（按轮安排工人）→ 分拣 → 分批发货（冷链）→ 认养人权益已发/在途/待发/余额同步 →
坏果/争议/赔付 → 保险理赔 → 成熟度回访 → **权益归档** → 最终交付结算 → 地块成本与客户满意度沉淀。

技术栈：**React 18（Vite）+ Go 1.19（net/http + pgx）+ PostgreSQL 15**，单镜像部署、`docker compose` 一键启动。

---

## 一、一键启动（Docker Compose，推荐）

前置：已安装 Docker 与 Docker Compose。

```bash
cd /workspace/135-feature-3-b
docker compose up --build -d
```

启动后访问：

- 平台首页（认养项目）：<http://localhost:8080>
- 健康检查：<http://localhost:8080/api/health>

服务说明：

| 服务 | 内容 | 端口 |
|---|---|---|
| `app` | Go 后端 API + 托管 React 构建产物（单端口同源） | 8080 |
| `db`  | PostgreSQL 15，库名 `farmcrowd`，用户/密码 `farm/farm123` | 5432 |

首次启动时后端会自动执行数据库迁移并写入演示数据（含冰雹减产、坏果投诉、临时改地址、
企业团购权益口径争议等完整场景），无需手工初始化。停止用 `docker compose down`，
清除数据卷用 `docker compose down -v`。

---

## 二、演示账号（密码均为 `farm123`）

| 角色 | 姓名 | 手机号 | 登录后主要视图 |
|---|---|---|---|
| 平台管理员 | 平台管理员 | `13800000000` | 平台运营总览、品类与结季数据沉淀 |
| 农场主 | 李远山 | `13900000001` | 农场工作台、地块完整账、发布项目/农事/采摘/发货/结算 |
| 认养人 | 陈曦 | `13900000002` | 我的认养、权益兑现核对、证书、改地址、回访 |
| 认养人（企业） | 周凯 | `13900000007` | 星辰科技 20 棵企业团购订单 |
| 认养人 | 吴琳 | `13900000008` | 苹果 10 份的**成熟不均分 3 轮发货方案正等待确认** |
| 采摘工 | 赵秀兰 | `13900000003` | 回填采摘实际产量与坏果率、按分轮任务出工 |
| 包装员 | 王梅 | `13900000004` | 登记分拣与包装规格、**确认分批采摘计划** |
| 冷链物流 | 马建国 | `13900000005` | 推进发货状态（采摘→包装→发车→送达） |
| 客服 | 孙悦 | `13900000006` | 工单协调、赔付登记、回访 |

登录页点击任意演示账号即可自动填充手机号。

---

## 三、推荐验证路径（对应题目要求）

1. **不登录**浏览首页 5 个认养项目（🍎苹果 / 🫐蓝莓 / 🍯蜂蜜 / 🌾稻田 / 🥬蔬菜）与"直播·公益·团购"。
2. 用 **认养人陈曦 `13900000002`** 登录"我的认养"：
   - 苹果订单可看到 **保底 40kg、分两批发货、冷链路线、认养证书**；
   - "权益兑现"区展示已收到 / 在途 / 赔付折算，冰雹保险赔付+补发按金额折回产量，
     区分"已公平兑现"与"含在途批次的预计兑现"；
   - 可查看坏果投诉工单中 6 个角色的完整处理流水。
3. 用 **农场主 `13900000001`** 登录"农场工作台" → 点击"东坡3号苹果园"查看**地块完整账**：
   六类成本、冰雹灾害、成熟度抽检、采摘、分拣、发货、赔付、回访一页到底。
4. 在任一进行中项目页用农场操作栏走一遍：**记灾害（自动建工单）→ 成熟度抽检 → 安排采摘 →
   采摘工账号回填 → 包装员分拣 → 编批发货 → 物流发车/送达 → 季末结算**。
5. **成熟度分批发货（本题主线）**，三种角色配合：
   - 用 **农场主 `13900000001`** 打开"东坡3号苹果园" → **成熟度分批**标签：可见为吴琳 10 份苹果
     编制的 3 轮方案（阳面头采 90kg 脆甜 / 中段过渡 80kg 微酸 / 内膛后熟 30kg 粉糯），
     也可点"🍎 成熟度分批"为任一订单**按成熟度拆多轮、逐轮写明预计重量与口感差异**，
     然后 **提交（采摘计划同步包装组）**；
   - 换 **包装员王梅 `13900000004`** 登录同一标签 → **✅ 包装组确认采摘计划**；
   - 换 **认养人吴琳 `13900000008`** 登录"我的认养"：苹果订单顶部出现**待确认横幅**，
     弹窗逐轮核对预计重量与口感差异后**确认**；确认后系统**同步生成每轮物流计划（运单）、
     采摘任务，并写入权益台账**；
   - 回到农场主：在方案卡片上为**每一轮安排采摘工人**（农场据此组织下一次采摘），
     用采摘工回填、再由物流把各轮运单依次发车/送达；认养人侧实时看到
     **已送达 / 在途 / 待发 / 权益余额**与逐轮权益流水；
   - 全部轮次送达、权益余额为 0 后，农场主点 **📦 权益归档**；未送达或余额未清时归档会被拦截。
6. 用 **客服 `13900000006`** 打开异常工单，追加各角色处理事件并"登记赔付并结案"，
   观察工单状态自动变为"已赔付"、赔付折算回认养人权益。
7. 用 **管理员 `13800000000`** 查看平台总览：品类销售、公益代捐份额、保险到账、结季成本与满意度。

已内置的完整故事线（批次 `APPLE-2026-A`）：

- 6 月冰雹 → 农场拍照报灾、保险报案与到账、客服向全部份额推送三选一方案；
- 首批坏果率偏高 → 认养人投诉、物流出示温控记录、包装员整改、农场补发、客服回访闭环；
- 认养人出差 → 第二批发货前改地址，客服审核、物流更新冷链路线；
- 企业团购"净重/毛重"口径争议 → 包装员核对规格、农场发下季抵扣券、平台修订标注口径；
- 同一批次成熟不均（阳面果已上糖、内膛果尚青）→ 平台按成熟度拆 3 轮发货并逐轮写明
  预计重量与口感差异 → 包装组确认采摘计划 → 认养人确认 → 运单/采摘任务/权益同步 →
  农场逐轮安排采摘工人 → 认养人看到已发/待发/权益余额 → 全部送达后权益归档；
- 最终交付记录沉淀产量、发货、赔付、满意度、续认率与下一季建议。

---

## 四、本地开发（不用 Docker）

需要：Go 1.19+、Node.js 20+、PostgreSQL 15。

### 1. 准备数据库

```bash
createdb farmcrowd
# 数据库表结构与演示数据由后端启动时自动迁移（backend/internal/db/migrations/*.sql）
```

### 2. 启动后端（:8080）

```bash
cd backend
export DATABASE_URL="postgres://用户:密码@127.0.0.1:5432/farmcrowd?sslmode=disable"
go run ./cmd/server
```

后端在无 `STATIC_DIR` 时只提供 API，适合前后端分离开发。

### 3. 启动前端（:5173）

```bash
cd frontend
npm install
npm run dev        # 已在 vite.config.js 中把 /api 代理到 http://127.0.0.1:8080
```

打开 <http://localhost:5173>。生产模式下也可只跑后端并让 Go 托管前端构建产物：

```bash
cd frontend && npm run build
cd ../backend && STATIC_DIR=../frontend/dist go run ./cmd/server   # 然后访问 :8080
```

### 4. 测试

```bash
cd backend && go test ./...        # 单元测试
```

端到端 API 验证脚本 `backend/scripts/e2e_test.py`（仅依赖 Python3 标准库，无第三方包），
**需在全新初始化的数据库上运行**（脚本会新建认养、工单、结算等数据）：

```bash
# 1. 用空库启动后端（迁移+种子会自动执行）
# 2. 运行 98 项端到端断言（默认目标 http://127.0.0.1:8080，可用 FARMCROWD_BASE 覆盖）
python3 backend/scripts/e2e_test.py
# 容器场景：FARMCROWD_BASE=http://localhost:8080 python3 backend/scripts/e2e_test.py
```

覆盖：七角色登录鉴权、认养付款/超卖、保底权益量化核对（已发货+赔付折算）、
改地址拦截与建工单、灾害自动工单、成熟度抽检、采摘回填权限、分拣、发货状态机与
非法跳转拦截、全部送达订单自动完成、坏果工单 5 角色流水、赔付自动结案、
保险报案→到账、回访评分校验、直播/参观/团购/公益、地块台账、看板、季末结算；
以及**成熟度分批发货完整链路**：多轮方案编制（逐轮重量+口感差异校验）、
draft→submitted→pack_confirmed→adopter_confirmed 顺序与角色越权拦截、
认养人确认自动同步运单/采摘任务/权益台账、逐轮安排采摘工、发车/送达写权益流水、
已送达/在途/待发/权益余额核对、未送达禁归档、全部送达后权益归档。

---

## 五、主要 API（JSON，除登录/浏览外均需 `Authorization: Bearer <token>`）

| 方法 & 路径 | 说明 |
|---|---|
| `POST /api/auth/login` `POST /api/auth/register` `GET /api/auth/me` | 登录/注册/当前用户 |
| `GET /api/projects` `GET /api/projects/{id}` `POST /api/projects` | 项目列表/聚合详情（农事+抽检+采摘+分拣+发货+工单+理赔+结算）/发布 |
| `POST /api/projects/{id}/logs` | 农事/施肥/用药/**灾害**日志（灾害自动建工单） |
| `POST /api/projects/{id}/checks` | 成熟度抽检（糖度、分级、坏果率、预估产量） |
| `POST /api/projects/{id}/harvests` `POST .../harvests/{hid}/complete` | 安排采摘 / 采摘工回填 |
| `POST /api/projects/{id}/sorting` | 分拣与包装规格 |
| `POST /api/projects/{id}/shipments` `POST .../shipments/{sid}/advance` | 分批发货编排 / 状态机 `planned→picked→packed→shipped→delivered` |
| `GET/POST /api/projects/{id}/delivery-plans` | 成熟度分批方案列表 / 编制（多轮，逐轮预计重量+口感差异） |
| `POST /api/delivery-plans/{id}/submit` `.../pack-confirm` `.../confirm` `.../archive` | 提交（同步包装组）/ 包装组确认采摘计划 / 认养人确认（同步运单·采摘任务·权益台账）/ 权益归档 |
| `POST /api/delivery-plans/{id}/rounds/{rid}/picker` | 农场为某一轮安排/改派采摘工人 |
| `POST /api/projects/{id}/costs` `POST .../settle` | 地块记账 / 季末最终结算 |
| `POST /api/adoptions` `GET /api/my/adoptions` `POST /api/adoptions/{id}/address` | 付款（自动发证书，支持公益代捐）/ 我的权益核对 / 临时改地址 |
| `GET/POST /api/incidents` `GET/POST /api/incidents/{id}` `POST /api/incidents/{id}/events` | 异常工单与多角色处理流水 |
| `POST /api/compensations/incident/{id}` | 登记现金/补发/保险/下季抵扣赔付 |
| `GET/POST /api/insurance-claims` `POST /api/insurance-claims/{id}` | 保险报案/查勘/到账（到账自动生成赔付） |
| `GET/POST /api/callbacks` | 成熟度回访（满意度+续认建议） |
| `GET /api/live` `POST /api/live/{id}` | 农场直播与开播流转 |
| `GET/POST /api/visits` `POST /api/visits/{id}` | 地块参观预约与审核 |
| `GET/POST /api/group-buys` `POST /api/group-buys/{id}` | 企业团购 |
| `GET /api/charity` | 公益认养榜 |
| `GET /api/plots/{id}/ledger` | **地块完整账**（投入→灾害→成熟→采摘→分拣→发货→售后→成本/满意度） |
| `GET /api/farm/dashboard` `GET /api/admin/overview` | 农场看板 / 平台总览 |

> 本项目为含前端业务页面的全栈应用，按任务约定不单独提供 Swagger/OpenAPI UI；
> 接口契约以上表与 `backend/internal/api` 源码为准。

---

## 六、数据模型要点

- 所有业务记录都携带 `project_id + batch_no`（认养批次号），这是把各方处理结果
  "串到同一认养批次"的主线；`incident_events` 以 `actor_role` 记录 6 类角色的每一步处置。
- `projects.guarantee_kg` 为每份保底产量；认养人权益 = 已发货量 + 赔付折算量（现金/抵扣按
  单价折回产量，补发按果品单价折算），在 `/api/my/adoptions` 的 `rights_fulfillment` 中量化呈现。
- `plot_costs` 六类成本（input/disaster/harvest/sorting/logistics/compensation）+
  `callbacks` 满意度/续认意愿 + `settlements` 最终交付，构成结季后的数据沉淀。
- **成熟度分批发货**三张表：`delivery_plans`（方案主表与状态机：draft→submitted→
  pack_confirmed→adopter_confirmed→archived）、`delivery_plan_rounds`（每轮预计重量、
  口感差异、成熟度说明、计划日期，并回链确认后生成的 `shipment_id` / `harvest_task_id`）、
  `rights_records`（认养人权益台账：期初保底 opening/排产 allocation/发车 dispatch/
  送达 delivery/赔付 compensation/归档 archive，逐笔带权益余额）。认养人确认方案即在一个
  事务内同步生成各轮运单与采摘任务、写入排产流水；`harvest_tasks.plan_round_id` 把采摘任务
  挂回分轮，农场据此逐轮安排采摘工人；运单发车/送达自动补记权益流水，全部送达且余额为零方可归档。

## 七、目录结构

```
.
├── docker-compose.yml          # db + app 一键启动
├── Dockerfile                  # 三阶段：node 构建前端 → go 构建后端 → alpine 运行
├── backend/
│   ├── cmd/server/main.go      # 入口（API + SPA 托管 + CORS + 自动迁移）
│   ├── internal/api/           # 路由与全部业务处理器（delivery.go 为成熟度分批发货）
│   └── internal/db/
│       ├── db.go               # 连接池、迁移运行器
│       └── migrations/         # 001 建表 / 002 演示数据 / 003-004 成熟度分批（go:embed）
└── frontend/
    └── src/
        ├── App.jsx             # 路由与角色化导航
        ├── api.js auth.jsx
        └── pages/              # 项目/详情/我的认养/工单/农场工作台/地块账/服务/平台总览
```

## 八、实际执行过的验证（本环境）

- `go build ./...`、`go vet ./...`、`gofmt -l`（无输出）：通过；`go test ./...`：通过（数值转换/折算单测）。
- 对真实 PostgreSQL 15（本环境用 `initdb` 在 `/tmp` 起的独立 5433 实例）加载
  `001/002/003/004` 迁移：**27 张表**创建成功，种子数据可重复执行（幂等），含成熟度分批
  2 个方案 / 4 个轮次 / 6 笔权益流水。
- 启动 Go 服务后执行端到端接口断言（真实 HTTP + 真实数据库，全新空库单次运行），
  **98/98 全部通过（exit 0）**，在原有 65 项基础上新增 33 项，新增覆盖：
  成熟度分批多轮编制与校验（至少 2 轮、逐轮必须写口感差异）、认养人无权编制、
  draft→submitted→pack_confirmed→adopter_confirmed 的顺序与采摘工/包装员/非本人越权拦截、
  认养人确认在一个事务内同步生成每轮运单与采摘任务并写排产权益流水、
  逐轮安排采摘工（非采摘工账号被拒、认养人无权）、第 1 轮送达后已送 20/待发 10/余额 10、
  发车/送达写权益流水、未全部送达禁止归档、两轮全部送达余额归零后权益归档、
  已归档蓝莓方案与待确认苹果 3 轮方案对认养人可见。
- 前端 `npm install`（本地缓存目录）+ `npm run build`（Vite）：构建成功（46 模块，
  JS ≈ 246KB / gzip ≈ 76KB）。
- 用 Go 后端 `STATIC_DIR` 托管前端产物验证：`/`、SPA 深链路 `/my`、静态资源、
  `/api/*` 同源均正常（均 200/401 符合预期）；公开项目详情中 `delivery_plans`
  只暴露分轮安排与确认状态，不含权益金额等隐私字段。
- 局限：本验证环境未安装 Docker 守护进程与无头浏览器，因此**未实际执行**
  `docker compose up` 的容器内联调与浏览器点击；Dockerfile 各构建步骤已分别用
  等价命令（npm ci/build、CGO 构建、alpine 健康检查命令）验证。
