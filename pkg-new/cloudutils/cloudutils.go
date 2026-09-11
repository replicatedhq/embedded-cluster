package cloudutils

import "github.com/sirupsen/logrus"

type CloudUtils struct {
	logger logrus.FieldLogger
}

type CloudUtilsOption func(*CloudUtils)

func New(opts ...CloudUtilsOption) *CloudUtils {
	c := &CloudUtils{}
	for _, opt := range opts {
		opt(c)
	}

	if c.logger == nil {
		c.logger = logrus.StandardLogger()
	}

	return c
}
