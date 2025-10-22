module github.com/your-org/croupier

go 1.21

require google.golang.org/grpc v1.65.0 // indirect

require github.com/cuihairu/croupier-sdk-go v0.0.0-00010101000000-000000000000 // indirect

replace github.com/cuihairu/croupier-sdk-go => ./sdks/go
