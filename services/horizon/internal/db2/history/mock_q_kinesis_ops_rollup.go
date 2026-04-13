package history

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockQKinesisOpsRollup is a mock implementation of the QKinesisOpsRollup interface
type MockQKinesisOpsRollup struct {
	mock.Mock
}

func (m *MockQKinesisOpsRollup) NewKinesisOpsRollupBatchInsertBuilder(maxBatchSize int) KinesisOpsRollupBatchInsertBuilder {
	a := m.Called(maxBatchSize)
	return a.Get(0).(KinesisOpsRollupBatchInsertBuilder)
}

// MockKinesisOpsRollupBatchInsertBuilder is a mock implementation of KinesisOpsRollupBatchInsertBuilder
type MockKinesisOpsRollupBatchInsertBuilder struct {
	mock.Mock
}

func (m *MockKinesisOpsRollupBatchInsertBuilder) Add(ctx context.Context, entry KinesisOpsRollup) error {
	a := m.Called(ctx, entry)
	return a.Error(0)
}

func (m *MockKinesisOpsRollupBatchInsertBuilder) Exec(ctx context.Context) error {
	a := m.Called(ctx)
	return a.Error(0)
}
