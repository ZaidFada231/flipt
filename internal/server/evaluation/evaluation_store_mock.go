package evaluation

import (
	"context"

	"github.com/stretchr/testify/mock"
	flipt "go.flipt.io/flipt/rpc/flipt"
	"go.flipt.io/flipt/rpc/flipt/data"
)

var _ Storer = &evaluationStoreMock{}

type evaluationStoreMock struct {
	mock.Mock
}

func (e *evaluationStoreMock) String() string {
	return "mock"
}

func (e *evaluationStoreMock) GetFlag(ctx context.Context, namespaceKey, key string) (*flipt.Flag, error) {
	args := e.Called(ctx, namespaceKey, key)
	return args.Get(0).(*flipt.Flag), args.Error(1)
}

func (e *evaluationStoreMock) GetEvaluationRules(ctx context.Context, namespaceKey string, flagKey string) ([]*data.EvaluationRule, error) {
	args := e.Called(ctx, namespaceKey, flagKey)
	return args.Get(0).([]*data.EvaluationRule), args.Error(1)
}

func (e *evaluationStoreMock) GetEvaluationDistributions(ctx context.Context, ruleID string) ([]*data.EvaluationDistribution, error) {
	args := e.Called(ctx, ruleID)
	return args.Get(0).([]*data.EvaluationDistribution), args.Error(1)
}

func (e *evaluationStoreMock) GetEvaluationRollouts(ctx context.Context, namespaceKey, flagKey string) ([]*data.EvaluationRollout, error) {
	args := e.Called(ctx, namespaceKey, flagKey)
	return args.Get(0).([]*data.EvaluationRollout), args.Error(1)
}
