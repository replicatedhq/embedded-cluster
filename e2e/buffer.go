package e2e

import "bytes"

type buffer struct {
	*bytes.Buffer
}

func (b *buffer) Close() error {
	return nil
}
