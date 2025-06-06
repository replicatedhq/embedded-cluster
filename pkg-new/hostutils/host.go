package hostutils

import (
	"github.com/sirupsen/logrus"
)

var _ HostUtilsInterface = (*HostUtils)(nil)

type HostUtils struct {
	logger logrus.FieldLogger
}

type HostUtilsOption func(*HostUtils)

func WithLogger(logger logrus.FieldLogger) HostUtilsOption {
	return func(h *HostUtils) {
		h.logger = logger
	}
}

func New(opts ...HostUtilsOption) *HostUtils {
	h := &HostUtils{
		logger: logrus.StandardLogger(),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}
