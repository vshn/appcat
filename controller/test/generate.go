package controllertest

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_client.go -package=mocks sigs.k8s.io/controller-runtime/pkg/client Client
