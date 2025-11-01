module github.com/cuihairu/croupier

go 1.24.0

require google.golang.org/grpc v1.76.0

require (
	github.com/cuihairu/croupier-sdk-go v0.0.0-20251101021458-00bf8657e89b
	github.com/spf13/cobra v1.7.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.18.2 // indirect
	golang.org/x/net v0.46.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.30.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251029180050-ab9386a59fda // indirect
	google.golang.org/protobuf v1.36.10
)

replace github.com/cuihairu/croupier-sdk-go => ./sdks/go
