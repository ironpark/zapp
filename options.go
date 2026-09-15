package zapp

import (
	"github.com/ironpark/zapp/pkg/signing"
	"time"
)

type Logger interface {
	Printf(string, ...any) (int, error)
}
type Option func(*options)
type options struct {
	signBackend, notaryBackend signing.Backend
	logger                     Logger
	clock                      time.Time
}

func WithLogger(l Logger) Option   { return func(o *options) { o.logger = l } }
func WithClock(t time.Time) Option { return func(o *options) { o.clock = t } }

// WithSigningBackend selects a caller-provided signing implementation.
func WithSigningBackend(b signing.Backend) Option { return func(o *options) { o.signBackend = b } }

// WithNotarizationBackend selects a caller-provided notary implementation.
func WithNotarizationBackend(b signing.Backend) Option {
	return func(o *options) { o.notaryBackend = b }
}
