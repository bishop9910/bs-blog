# bs-blog

一个人的写作空间。前台是手写 CSS 的阅读站点，后台是一个带密码锁的富文本编辑器。

- **前端**：Vue 3 + Vite + TypeScript，动效用 anime.js v4，根目录
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

```bash
bash backend/scripts/build-release.sh
```

两个脚本做的事完全一样，按平台挑一个：Windows 用 `.ps1`，Linux / macOS 用 `.sh`。一条命令走完：`pnpm build` 产出前端 → 复制进 `backend/internal/web/dist/` → 用 `go:embed` 把前端**编进二进制** → 在 `backend/release/` 下组装交付物：

```
release/
├── bs-blog.exe        前端已内嵌，单个文件即可运行（Linux 上叫 bs-blog）
├── configs/config.yaml
└── data/uploads/      图片上传目录
```

整个目录拷到别的机器就能跑——**不需要 Node，也不需要 dist 目录**，在 `release/` 下执行 `.\bs-blog.exe`（Linux 上是 `./bs-blog`），浏览器开 <http://127.0.0.1:8080>。

| 用途 | `.ps1` | `.sh` |
| --- | --- | --- |
| 指定版本号 | `-Version 1.2.3` | `-v 1.2.3` |
| 跳过前端构建 | `-SkipFrontend` | `-s` |

版本号默认取 `git describe --tags --always --dirty`，会打进启动日志。

`.sh` 还认 `GOOS` / `GOARCH` 环境变量，所以在 Linux 上交叉编 Windows 版本也是可以的：`GOOS=windows bash backend/scripts/build-release.sh`，产物名会自动变成 `bs-blog.exe`。反过来，在 Linux 上直接跑出来的是一个**静态**二进制（`CGO_ENABLED=0`，SQLite 驱动是纯 Go 的），拷到任何 amd64 Linux 都能直接跑。

**`release/data/` 不会被脚本覆盖。** 数据库是唯一不可再生的东西，脚本只建空目录；如果 `backend/data/blog.db` 存在而 `release/data/` 里没有，它只打一行 NOTE 提醒你手动拷贝，绝不代劳。

### Windows 图标与版本信息

Windows 产物自带图标和版本元数据（资源管理器里「属性 → 详细信息」能看到），这些资源由 `backend/winres/` 管：

- `winres/icon.ico` 就是 exe 的图标，而且**是个普通的受版本控制的文件**：想换图标就把这个文件替换掉，提交后一直保留，打包脚本不会动它
- `winres/versioninfo.json` 是版本元数据（产品名、描述、版权、占位版本号）
- [goversioninfo](https://github.com/josephspurrier/goversioninfo) 把这两样编成 `.syso`，Go 再把它链进 exe

`tools/icongen` 只是 `icon.ico` 的**兜底**：万一文件不在（比如新克隆时漏带了），它就用 Go 代码现画一个——圆角底加一个字母 `b`，画在 1024² 的设计空间里再降采样到 16/24/32/48/64/128/256 七个尺寸。它**默认绝不覆盖已存在的文件**，想重画必须显式加 `-force`；打包脚本看到 `icon.ico` 已经在了就连碰都不碰它：

```sh
cd backend
go run ./tools/icongen          # 只在 icon.ico 缺失时补一个，有就原样留着
go run ./tools/icongen -force   # 重画并覆盖
```

仓库里预置的 `icon.ico` 是从前端 `public/favicon.ico` 拷过来的，这样 exe 图标和网页标签图标是同一个；换图标时记得两处都换。想用别处的图标，只要是个真 `.ico`（goversioninfo 不吃 `.png`，会报 `bad magic number`）直接覆盖上去即可。

goversioninfo 是**外部工具**，装一次即可：

```sh
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
```

没装也能正常打包，只是产物没有图标和版本标签（脚本会打一行 NOTE 说明）。版本号能解析出 `1.2.3` 时会同时写进资源管理器显示的**数值**版本；解析不出来（比如 `1daaee3-dirty` 这种提交哈希）就只覆盖文本字段，数值保留 `versioninfo.json` 里的占位值。

交叉编译到非 Windows 目标时这一步自动跳过：`.syso` 只在 Windows 上有意义，而 goversioninfo 生成的 `resource_windows_*.syso` 这个命名本身也保证它们不会被链进 Linux 二进制——即使 `cmd/server/` 里还留着上一次 Windows 构建的残留文件也不会。

前端是**直接从二进制里读出来**服务的（`internal/web/embed.go`），不做「启动时解压到临时目录」——少一次写盘、少一个清理步骤，也就少一类「临时目录不可写」的启动失败。代价是内嵌文件没有修改时间，所以缓存头是显式写死的：`index.html` 用 `no-cache`（它决定引用哪些带哈希的资源，缓存它等于把浏览器钉在旧构建上），`assets/` 用一年 `immutable`。

开发时仍然前后端分开跑（见上面的快速开始）。想临时覆盖内嵌前端，把 `server.staticDir` 指向一个目录即可；指向的路径不存在或没有 `index.html` 时会**自动回退到内嵌副本**，所以配错不会把站点打挂。

要单独手工构建也可以：

```sh
pnpm build                            # 前端产物到 dist/
cd backend
go build -o bs-blog ./cmd/server      # 从 backend/ 启动时会去读 ../dist
```

这样编出来的二进制**没有内嵌前端**（只嵌了占位的 `.gitkeep`），启动时会打印一条「serving the API only」的警告；要出页面就得让 `server.staticDir` 指向 `../dist`。

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

打包用的 `backend/scripts/build-release.ps1`（Linux 上是 `build-release.sh`）不在这四个里面：它们只读前端产物、只写 `backend/release/`，既不碰数据库也不起服务，所以不需要在测试前跑。

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
- 文章详情：跟着正文走的阅读进度条、右侧吸顶目录（窄屏折叠成一张可展开的卡片）、回到顶部、代码高亮、上一篇/下一篇、copy link
- 归档：按年/月分组，年份栏吸顶
- 标签、关于、404（打字机动画）
- 壁纸：后台配好一组之后，**每次打开站点随机挑一张**，电影感的淡入 + 缓慢推镜 + 跟随鼠标与滚动的视差；配了多张时每 45 秒自动交叉淡入换下一张（页面切到后台就停下）；没有配壁纸时保持原来的极光背景
- 音乐：侧边可展开/收起的播放器，左边封面、中间歌词、进度条上方是**正在播放的声音**实时频谱，支持上一首/下一首/顺序/单曲/随机/播放列表，空格键播放暂停，系统媒体键（MediaSession）与手机锁屏也能控制
- 双语歌词：同一行时间戳上写第二行就是译文，播放器把原文排在上面、译文用小字排在下面，跟着当前行一起高亮（日语歌配中文翻译是这么排的）

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

**音乐库**

音乐库就是**一个目录**，没有任何一首歌进数据库——把文件搬进去或搬走，站点播的就是那些文件：

- 递归扫描 `music.dir`（默认 `backend/data/music`），索引 mp3 / flac / m4a / m4b / ogg / opus / wav
- 标签与内嵌封面直接读文件（ID3v2 / Vorbis / FLAC / MP4），**时长是纯 Go 解析的**：不需要 ffmpeg，也不需要 CGO
- 歌词优先取文件内嵌的，其次取同名 `.lrc`（UTF-8 或 GB18030 都能读，`[offset:]` 会被应用）；同一时间戳连续写多行时，第一行当原文、其余当译文
- 同目录放一个与音频同名的 `.json`（`{title,artist,album,albumArtist,year,cover,lyrics}`）可以覆盖标签，`cover` 是相对音频文件的图片路径
- 丢进去之后 `music.watch` 会在 2 秒防抖后自动重扫；也可以在后台「音乐」页点**重新扫描**，或直接拖放上传
- 播放走 `GET /api/v1/music/stream/{id}`（支持 Range，拖进度条不会重下整首），封面走 `/api/v1/music/cover/{id}` 并缓存一天

前端是**同源**取音频的，所以 `AnalyserNode` 能直接读正在播放的声音——进度条上方那排频谱不是随机数，是当前这首歌的实时频率。

---

## 动效

站点的 JS 动效统一经过 `src/utils/anime.ts`——anime.js v4 在整个前端**只在这一个文件里被 import**。这么收口是为了三条规则没法在调用点被忘记：

- **`prefers-reduced-motion` 优先**：每个封装在访客要求减少动效时直接返回 `null`，动画完全不执行。所以调用方必须保证「不执行时页面已经是最终状态」——**没有任何元素是靠 CSS 预先藏起来再等 JS 显示的**，动画只做加法，JS 出问题内容也照样在。
- **缓动对齐设计令牌**：默认缓动 `outExpo` 就是 CSS 里的 `--ease-out`，JS 动画与 CSS 过渡不会是两种材质。
- **升级只改一处**：v3 到 v4 的 API 变动很大（`text` 这类属性在 v4 里会落到 attribute 而看不见，必须写 `innerHTML`），换版本时不必翻全站。

导出的原语：`motion()`（`animate()` 的降级安全版）、`motionTimeline()`、`motionStagger()`（列表错峰）、`motionOnEnter()`（滚到视口才播）、`scrambleTo()`（文字乱码揭示）、`countUp()`（数字滚动）、`springEase()`，以及 `animate` / `createAnimatable` / `onScroll` / `stagger` / `utils` 的透传。

用在哪儿：页头站名的乱码揭示（路由切换会重放）、首页 hero 与统计数字、文章卡片错峰入场、壁纸轮换的交叉淡入、播放器封面旋转与声音脉冲/面板编排、阅读进度条的阻尼跟随。它们和 `v-reveal` 指令（`IntersectionObserver` + CSS 过渡）是两套东西，**同一个元素只归一个机制管**，避免两套动画抢同一个属性。

### 路由切换

公开站在 `PublicLayout.vue` 里用**方向感知**的转场（`<Transition :css="false">` 配 anime.js 的 leave/enter 钩子）替代了原来的 `route-fade` 类名过渡：每个公开路由在 meta 里声明 `order`（首页 0、文章 1、文章详情 2、归档 3、标签 4、标签详情 5、日历 6、关于 7），页内往前导航就从**右侧推入**、旧页往左上退出（200ms 走、420ms 进），往回导航整体镜像——所以「回到归档」不会看起来像「又去了一次归档」。转场的同时一道 `--accent` 色的光带横向掠过一次（`z-index: var(--z-overlay)`，暗色 pink-500、浅色 pink-600，跟着主题自动换），它挂在被切换的视图**之外**，页面交换不会把它一起带走。

两个必须保留的护栏：动画被下一次导航打断时 anime.js 不再触发 `onComplete`，所以 `done` 有看门狗兜底（否则路由视图要永久卡在旧页面）；进场结束后必须清掉 anime.js 留下的内联 `transform`，否则这个元素就成了 `position: fixed` 的包含块，阅读进度条和右侧目录会跟着错位。管理后台仍是轻量的 CSS 淡入淡出——后台是工作区，每次点击都演 600ms 会碍事。


---

## 目录结构

```
├── src/                     前端
│   ├── api/                 接口封装（唯一发出请求的地方）
│   ├── components/          公共组件、编辑器、可视化效果、播放器、阅读进度与目录
│   ├── composables/         主题、平滑滚动、代码高亮、打字机、站点信息、壁纸、播放器
│   ├── layouts/             PublicLayout / AdminLayout
│   ├── router/              路由与登录守卫
│   ├── stores/auth.ts       登录状态（模块级单例，无需 Pinia）
│   ├── styles/              设计令牌 + 基础样式 + 动效 + 编辑器样式
│   ├── types/blog.ts        前后端共享的数据契约
│   ├── utils/               格式化、文本处理、HTML 净化、LRC 解析、anime.js 动效封装
│   └── views/               页面
├── backend/
│   ├── cmd/server/          入口
│   ├── configs/config.yaml  配置（全部带注释）
│   ├── data/music/          音乐库（运行时创建，已 gitignore）：丢音频进来即可
│   ├── scripts/             端到端测试脚本 + 打包脚本
│   ├── tools/icongen/       图标兜底生成器：icon.ico 缺失时才用 Go 代码画一个（纯标准库）
│   ├── winres/              Windows 资源：versioninfo.json 与 icon.ico，都是手改的源文件
│   ├── release/             打包产物（已 gitignore）
│   └── internal/
│       ├── biz/             业务逻辑与线上 DTO
│       ├── conf/            配置加载与默认值
│       ├── data/            数据访问（GORM）+ 音乐库索引
│       ├── data/audio/      纯 Go 音频时长/标签解析（无 CGO，移植自 achero_server）
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
- 音频上传做**扩展名白名单 + MIME 嗅探**双校验，落盘名是随机 UUID，原始文件名不会碰到文件系统
- 音乐 sidecar（`.json` / `.lrc` / 封面）里的路径必须落在音乐目录内部，`../` 之类的越界引用会被拒绝，不会被当成「读我这个博客服务器上的任意文件」
- 壁纸只允许站点相对路径或 http(s) 绝对地址；`javascript:`、`data:`、`//host` 都会被丢掉（前端再按同一套白名单校验一次）
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
| `music.dir` | `data/music` | 音乐库目录，递归扫描。**留空即关闭音乐库**（接口返回空列表，不会 404）；目录不存在也会自动退化 |
| `music.watch` | `true` | 目录变化后自动重扫；放在网络共享上被事件刷屏时可以关掉，改用后台的手动重扫 |
| `music.extensions` | 七种常见格式 | 参与索引的扩展名，去掉哪个就不再索引哪种格式（文件不会被删） |
| `music.maxUploadBytes` | `33554432` | 单个音频上传上限，**必须 ≤ `server.maxBodyBytes`**，否则启动时直接报配置错误 |
| `server.maxBodyBytes` | `33554432` | 请求体总上限；想传更大的无损音频，这两个键要一起调大 |
| `auth.tokenTtl` | `168h` | 登录有效期 |
| `log.level` | `info` | 改成 `debug` 会同时打印 SQL |

---

## 开发约定

- 前后端的数据契约以 `src/types/blog.ts` 为准，后端 `internal/biz/dto.go` 逐字段镜像它
- 后端路由是**手写**的，没有引入 protobuf 代码生成。受保护的路由通过 `h.authed(...)` 包装，鉴权在类型层面无法被静默漏掉
- 音乐路由和 `/uploads` 一样是**公开只读**的（听歌不该先登录），写操作（上传/删除/重扫）才要求会话
- 壁纸存在 `site_profile.wallpapers` 里，是一段 JSON 文本；只接受站点相对路径或 http(s) 绝对地址，最多 24 张、同址去重，空数组即清空。前端按**每次整页加载**随机挑一张，客户端路由切换不会重抽
- JS 动效一律走 `src/utils/anime.ts`，业务代码不直接 `import 'animejs'`；被动画的元素**不得**由 CSS 预设隐藏，`prefers-reduced-motion` 下页面必须原样可读
- 前端所有请求都经过 `src/api/http.ts`，401 会统一清掉本地 token 并跳回登录页
