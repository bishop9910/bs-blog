# bs-blog

一个人的写作空间。前台是手写 CSS 的阅读站点，后台是一个带密码锁的富文本编辑器。

- **前端**：Vue 3 + Vite + TypeScript，根目录
- **后端**：Go + Kratos v2 + SQLite（纯 Go 驱动，无需 CGO），`backend/`

前端与后端通过 `/api/v1` 下的 JSON 通信，字段一律 camelCase。生产环境由 Go 直接托管前端构建产物，**同源部署，因此不需要 CORS**。

---

## 快速开始

### 1. 安装前端依赖

```sh
pnpm install
```

### 2. 启动后端

```sh
cd backend
go run ./cmd/server
```

第一次启动会自动建库、建表，并在 `backend/data/` 下生成 SQLite 文件与 JWT 密钥。默认监听 `0.0.0.0:8080`，也就是本机用 <http://127.0.0.1:8080> 访问，同一局域网的其他设备用 <http://本机IP:8080> 访问。只想本机可用的话，把 `server.addr` 改回 `127.0.0.1:8080`。

### 3. 启动前端开发服务器

```sh
pnpm dev
```

打开 <http://localhost:5173>。Vite 已把 `/api` 与 `/uploads` 代理到 `127.0.0.1:8080`，开发期同样是同源体验。

### 4. 认领你的站点

首次打开 `/admin/login` 时，后端还没有任何用户，页面会自动切换到「初始化」模式，让你设置唯一的管理员账号和密码。**这个入口在站点建立第一个用户之后就会永久关闭**（再调用会返回 403），所以不用担心别人抢先注册。

---

## 生产构建

```powershell
powershell -File backend/scripts/build-release.ps1
```

一条命令走完：`pnpm build` 产出前端 → 复制进 `backend/internal/web/dist/` → 用 `go:embed` 把前端**编进二进制** → 在 `backend/release/` 下组装交付物：

```
release/
├── bs-blog.exe        前端已内嵌，单个文件即可运行
├── configs/config.yaml
└── data/uploads/      图片上传目录
```

整个目录拷到别的机器就能跑——**不需要 Node，也不需要 dist 目录**，在 `release/` 下执行 `.\bs-blog.exe`，浏览器开 <http://127.0.0.1:8080>。

`-Version 1.2.3` 指定版本号（默认取 `git describe --tags --always --dirty`，会打进启动日志）；`-SkipFrontend` 跳过 `pnpm build`，复用已有的 `dist/`。

**`release/data/` 不会被脚本覆盖。** 数据库是唯一不可再生的东西，脚本只建空目录；如果 `backend/data/blog.db` 存在而 `release/data/` 里没有，它只打一行 NOTE 提醒你手动拷贝，绝不代劳。

前端是**直接从二进制里读出来**服务的（`internal/web/embed.go`），不做「启动时解压到临时目录」——少一次写盘、少一个清理步骤，也就少一类「临时目录不可写」的启动失败。代价是内嵌文件没有修改时间，所以缓存头是显式写死的：`index.html` 用 `no-cache`（它决定引用哪些带哈希的资源，缓存它等于把浏览器钉在旧构建上），`assets/` 用一年 `immutable`。

开发时仍然前后端分开跑（见上面的快速开始）。想临时覆盖内嵌前端，把 `server.staticDir` 指向一个目录即可；指向的路径不存在或没有 `index.html` 时会**自动回退到内嵌副本**，所以配错不会把站点打挂。

要单独手工构建也可以：

```sh
pnpm build                        # 前端产物到 dist/
cd backend
go build -o bs-blog.exe ./cmd/server   # 从 backend/ 启动时会去读 ../dist
```

---

## 测试

后端带四个端到端测试脚本（Windows PowerShell，不依赖任何外部工具）：

```powershell
powershell -File backend/scripts/smoke.ps1   # 接口：鉴权、文章 CRUD、发布、上传、改密、CORS
powershell -File backend/scripts/web.ps1     # 生产托管：SPA 回退、路径穿越防护、目录列表
powershell -File backend/scripts/prod.ps1    # 真实整合：用 dist/ 真产物跑通前后端
powershell -File backend/scripts/dev.ps1     # 开发回路：vite dev + /api 代理是否接通
```

它们会在 `.smoke/` / `.smokeweb/` / `.smokeprod/` / `.smokedev/` 下起一次性服务和一次性数据库，跑完自动清理，**不会碰到真实数据**。

打包用的 `backend/scripts/build-release.ps1` 不在这四个里面：它只读前端产物、只写 `backend/release/`，既不碰数据库也不起服务，所以不需要在测试前跑。

前三个脚本的区别：`smoke.ps1` 只管接口；`web.ps1` 用一个手写的假 `dist/` 验证托管逻辑；`prod.ps1` 用 `pnpm build` 真正产出的 `dist/`，会解析 `index.html` 里引用的每一个资源并逐个请求，能抓到「页面指向了一个 Vite 根本没生成的文件」这类假 `dist/` 永远看不见的问题。**先跑 `pnpm build` 再跑 `prod.ps1`。**

`dev.ps1` 覆盖的是另一条完全独立的代码路径：`vite build` 通过**不能**证明 `vite dev`、代理配置和按需 SFC 编译也没问题。它会同时拉起后端和 Vite，验证 `/api` 代理接通、几个关键 SFC 能被正确编译、且 Vite 没记录任何错误。跑它会占用 5173/8080 端口。

端口被占用时不会静默通过：`dev.ps1` 会在启动前先探测，发现占用直接报 `port 8080 is already in use -- stop the running dev server first` 并以 1 退出；`smoke.ps1` 可以用 `-Port 8093` 换一个端口，方便和正在运行的开发服务器共存。

前端用 `pnpm type-check` 做类型检查。

### 前后端文本契约

slug、字数、阅读时长这三样东西**两边各算一次**：编辑器里算出来给你看，服务端保存时再算一次写进库。两边一旦有分歧，你看到的就是一个 slug，存下去的是另一个。

`scripts/text-cases.json` 是唯一的真值来源，两侧各有一份断言：

```sh
pnpm check:text                                    # 前端 src/utils/text.ts
cd backend && go test ./internal/pkg/textutil/     # 后端 internal/pkg/textutil
```

**改任意一边都必须改 `scripts/text-cases.json`**，否则另一边立刻红。`go test ./...` 里唯一有测试的包就是它。

### 排查库里的真实数据

`backend/cmd/dbdump` 是个只读小工具，用来在没装 SQLite 客户端时看一眼库里到底存了什么——「文章到底没存进去，还是存进去了只是没显示」这类问题一眼可辨：

```sh
cd backend && go run ./cmd/dbdump ../data/blog.db
```

它以 `mode=ro&_pragma=query_only(true)` 打开文件，**不可能写入**。库跑在 WAL 模式下，所以要看最新的已提交数据，需把 `blog.db`、`blog.db-wal`、`blog.db-shm` 三个文件一起复制出来再读——只复制 `blog.db` 会看到一份陈旧快照。

---

## 功能

**公开站点**

- 首页：极光 WebGL 背景、统计条、Bento 网格文章流、标签云
- 文章列表：分页、按标签筛选、关键字搜索（筛选状态写进 URL，可分享可后退）
- 文章详情：阅读进度条、代码高亮、上一篇/下一篇、copy link
- 归档：按年/月分组，年份栏吸顶
- 标签、关于、404（打字机动画）

**管理后台**（`/admin`，需要登录）

- 富文本编辑器：标题/正文、加粗斜体删除线、行内代码、引用、分割线、三种标题、有序无序列表、任务列表、表格、文本对齐、高亮、颜色、链接、图片、撤销重做、字数统计
- 图片：拖放或粘贴直接上传，也支持图片库统一管理
- 标签：输入即创建，按 slug 去重
- 草稿 / 发布：新文章**永远是草稿**，发布是一个单独的动作，半成品不可能误上线
- slug：从标题自动派生（中文标题也能生成可读 slug），手动改过之后就不再自动覆盖，实时检测占用
- 媒体库、站点信息（标题/介绍/社交链接）、修改密码

**密码锁**

- 单管理员。密码用 bcrypt cost 12 存储，登录换取 HS256 JWT
- 改密码会同时递增 token 版本号，让**所有旧 token 立即失效**
- 登录失败返回统一提示，不区分「用户不存在」和「密码错误」

---

## 目录结构

```
├── src/                     前端
│   ├── api/                 接口封装（唯一发出请求的地方）
│   ├── components/          公共组件、编辑器、可视化效果
│   ├── composables/         主题、平滑滚动、代码高亮、打字机、站点信息
│   ├── layouts/             PublicLayout / AdminLayout
│   ├── router/              路由与登录守卫
│   ├── stores/auth.ts       登录状态（模块级单例，无需 Pinia）
│   ├── styles/              设计令牌 + 基础样式 + 动效 + 编辑器样式
│   ├── types/blog.ts        前后端共享的数据契约
│   ├── utils/               格式化、文本处理、HTML 净化
│   └── views/               页面
├── backend/
│   ├── cmd/server/          入口
│   ├── configs/config.yaml  配置（全部带注释）
│   ├── scripts/             端到端测试脚本 + 打包脚本
│   ├── release/             打包产物（已 gitignore）
│   └── internal/
│       ├── biz/             业务逻辑与线上 DTO
│       ├── conf/            配置加载与默认值
│       ├── data/            数据访问（GORM）
│       ├── model/           表结构
│       ├── pkg/             文本、密码学、HTML 净化工具
│       ├── server/          HTTP 服务器、静态文件、错误页
│       ├── service/         路由与请求处理
│       └── web/             前端产物入库位置（go:embed；只有 .gitkeep 进版本控制）
└── dist/                    前端构建产物（Vite 产出，打包时被复制进 internal/web/dist）
```

---

## 安全设计

- 富文本在**入库前**用白名单策略净化（Go 侧 bluemonday），前端渲染时再用 DOMPurify 净一次；`javascript:` 之类的 URL 会被丢弃
- 数据库里存的是**净化后的 HTML**，公开页因此不需要启动编辑器就能渲染
- 文章正文不含 `data:` URI，图片必须走上传流程
- 单个请求体、上传体积都有上限；上传目录关闭目录列表
- 静态文件回退使用 `path.Clean("/" + path)`，`..` 无法逃出根目录
- JWT 密钥首次启动时随机生成并落盘（`backend/data/jwt.secret`），重启不会让所有人掉线。**`auth.secret` 必须留空**：`configs/config.yaml` 是提交进 git 的，密钥写在那里等于公开密钥——任何拿到仓库副本的人都能自己签一个管理员 token，直接绕过密码锁（`data/` 已被 `.gitignore` 排除，生成的密钥不会进仓库）
- 首次启动的认领接口 `POST /auth/setup` 在**还没有任何管理员**时对谁开放，谁先访问谁就是站长。`conf.Auth.SetupToken` 是为「认领时需要共享口令」预留的字段，**目前没有任何代码读取它**，所以配置里不放这个键——放一个不生效的值，看起来像一层并不存在的防护

---

## 配置

所有可调项都在 `backend/configs/config.yaml`，每一项都有注释。几个值得注意的：

| 键 | 默认 | 说明 |
| --- | --- | --- |
| `server.addr` | `0.0.0.0:8080` | 绑所有网卡，局域网内其他机器可用 `http://<本机IP>:8080` 访问；改成 `127.0.0.1:8080` 则只允许本机 |
| `server.staticDir` | 空 | 覆盖二进制内嵌的前端。留空就用内嵌副本；指向的目录不存在或没有 `index.html` 也回退到内嵌副本。想用外置目录（例如开发时指向 `../dist`）才需要填 |
| `data.uploadUrl` | `/uploads` | 已存文章的图片链接依赖这个前缀，改了会断图 |
| `data.maxUploadBytes` | `10485760` | 单张图片上限 |
| `auth.tokenTtl` | `168h` | 登录有效期 |
| `log.level` | `info` | 改成 `debug` 会同时打印 SQL |

---

## 开发约定

- 前后端的数据契约以 `src/types/blog.ts` 为准，后端 `internal/biz/dto.go` 逐字段镜像它
- 后端路由是**手写**的，没有引入 protobuf 代码生成。受保护的路由通过 `h.authed(...)` 包装，鉴权在类型层面无法被静默漏掉
- 前端所有请求都经过 `src/api/http.ts`，401 会统一清掉本地 token 并跳回登录页
