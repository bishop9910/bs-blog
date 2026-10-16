module github.com/bishop9910/bs-blog/backend

go 1.25.0

require (
	github.com/dhowden/tag v0.0.0-20240417053706-3d75831295e8
	github.com/fsnotify/fsnotify v1.10.1
	github.com/glebarez/sqlite v1.11.0
	github.com/go-kratos/kratos/v2 v2.9.2
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/microcosm-cc/bluemonday v1.0.27
	golang.org/x/crypto v0.55.0
	gopkg.in/yaml.v3 v3.0.1
	gorm.io/gorm v1.31.2
)

require (
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

// bluemonday v1.0.27 asks for x/net v0.26.0; only v0.33.0+ are cached here, so
// it is pinned forward too.
require golang.org/x/net v0.57.0 // indirect

// Pinned forward to releases that are actually present in the local module
// cache. kratos v2.9.2 asks for older ones (grpc v1.61.1, uuid v1.4.0,
// form/v4 v4.2.0, genproto rpc 2024-01) whose archives are not cached here, so
// minimum version selection is nudged to cached equivalents.
require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/glebarez/go-sqlite v1.21.2 // indirect
	github.com/go-kratos/aegis v0.2.0 // indirect
	github.com/go-playground/form/v4 v4.2.1 // indirect
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/text v0.41.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250811160224-6b04f9b4fc78 // indirect
	google.golang.org/grpc v1.65.0 // indirect
	google.golang.org/protobuf v1.36.7 // indirect
	modernc.org/libc v1.22.5 // indirect
	modernc.org/mathutil v1.5.0 // indirect
	modernc.org/memory v1.5.0 // indirect
	modernc.org/sqlite v1.23.1 // indirect
)
