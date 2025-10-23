module github.com/cuihairu/croupier

go 1.21

require google.golang.org/grpc v1.65.0

require (
	github.com/cuihairu/croupier-sdk-go v0.0.0-00010101000000-000000000000
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240528184218-531527333157 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
)

replace github.com/cuihairu/croupier-sdk-go => ./sdks/go
