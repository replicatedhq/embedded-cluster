package metrics

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/metrics/types"
	"github.com/stretchr/testify/mock"
)

var _ SenderInterface = (*MockSender)(nil)

// MockSender is a mock implementation of the SenderInterface
type MockSender struct {
	mock.Mock
}

// Send mocks the Send method
func (m *MockSender) Send(ctx context.Context, baseURL string, ev types.Event) {
	m.Called(ctx, baseURL, ev)
}
