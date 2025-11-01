module github.com/cuihairu/croupier

go 1.24.0

require (
	github.com/jackc/pgx/v5 v5.7.6
	google.golang.org/grpc v1.76.0
)

require (
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pelletier/go-toml/v2 v2.1.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	golang.org/x/crypto v0.43.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

require (
	github.com/cuihairu/croupier-sdk-go v0.0.0-20251101021458-00bf8657e89b
	github.com/spf13/cobra v1.7.0
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.18.2
	golang.org/x/net v0.46.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.30.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251029180050-ab9386a59fda // indirect
	google.golang.org/protobuf v1.36.10
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
)

replace github.com/cuihairu/croupier-sdk-go => ./sdks/go
