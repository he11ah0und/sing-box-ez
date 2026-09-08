module sing-box-ez

go 1.26.5

require (
	github.com/godbus/dbus/v5 v5.2.2
	github.com/gogpu/systray v0.3.0
	github.com/gorilla/websocket v1.5.3
	github.com/he11ah0und/config v0.2.2
	github.com/he11ah0und/localengine v0.3.2
	github.com/he11ah0und/logger v0.1.2
	github.com/vmihailenco/msgpack/v5 v5.4.1
	github.com/wailsapp/wails/v3 v3.0.0-beta.16
	github.com/yuin/gopher-lua v1.1.2
	golang.org/x/mod v0.38.0
	golang.org/x/sys v0.47.0
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.12
	gopkg.in/natefinch/npipe.v2 v2.0.0-20160621034901-c1b8fa8bdcce
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/coder/websocket v1.8.14 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-webgpu/goffi v0.6.3 // indirect
	github.com/he11ah0und/projectspec v0.1.1
	github.com/he11ah0und/yamltree v0.1.2 // indirect
	github.com/jchv/go-winloader v0.0.0-20250406163304-c1995be93bd1 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260904194346-d0f1323225a4 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

replace github.com/he11ah0und/localengine => ../localengine

replace github.com/he11ah0und/logger => ../logger

replace github.com/he11ah0und/projectspec => ../projectspec
