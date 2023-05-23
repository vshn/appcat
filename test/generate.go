package controllertest

// mock k8s client
//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_client.go -package=mocks sigs.k8s.io/controller-runtime/pkg/client Client

// mock vshnpostgresqls
//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_vshnpostgresqls.go -package=mocks -source=../pkg/apiserver/vshn/postgres/vshnpostgresql.go

// mock sgbackups
//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_sgbackups.go -package=mocks -source=../pkg/apiserver/vshn/postgres/sgbackups.go

// mock composition
//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_composition.go -package=mocks -source=../pkg/apiserver/appcat/composition.go
