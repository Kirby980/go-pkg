package saramax

//go:generate mockgen -source=./types.go -package=eventmock -destination=./mocks/article_consumer.mock.go Consumer
type Consumer interface {
	Start() error
}
