package history

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockClaimableBalanceClaimantBatchInsertBuilder struct {
	mock.Mock
}

func (m *MockClaimableBalanceClaimantBatchInsertBuilder) Add(ctx context.Context, claimableBalanceClaimant ClaimableBalanceClaimant) error {
	a := m.Called(ctx, claimableBalanceClaimant)
	return a.Error(0)
}

func (m *MockClaimableBalanceClaimantBatchInsertBuilder) Exec(ctx context.Context) error {
	a := m.Called(ctx)
	return a.Error(0)
}
