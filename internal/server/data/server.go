package data

import (
	"context"
	"fmt"

	"go.flipt.io/flipt/internal/storage"
	"go.flipt.io/flipt/rpc/flipt"
	"go.flipt.io/flipt/rpc/flipt/data"
	"go.uber.org/zap"
)

type EvaluationStore interface {
	ListFlags(ctx context.Context, namespaceKey string, opts ...storage.QueryOption) (storage.ResultSet[*flipt.Flag], error)
	storage.EvaluationStore
}

type Server struct {
	logger *zap.Logger
	store  EvaluationStore

	data.UnimplementedDataServiceServer
}

func NewServer(logger *zap.Logger, store EvaluationStore) *Server {
	return &Server{
		logger: logger,
		store:  store,
	}
}

func (srv *Server) EvaluationSnapshotNamespace(ctx context.Context, r *data.EvaluationNamespaceSnapshotRequest) (*data.EvaluationNamespaceSnapshot, error) {
	var (
		namespaceKey = r.Key
		resp         = &data.EvaluationNamespaceSnapshot{}
		remaining    = true
		nextPage     string
	)

	//  flags/variants in batches
	for remaining {
		res, err := srv.store.ListFlags(
			ctx,
			namespaceKey,
			storage.WithPageToken(nextPage),
		)
		if err != nil {
			return nil, fmt.Errorf("getting flags: %w", err)
		}

		flags := res.Results
		nextPage = res.NextPageToken
		remaining = nextPage != ""

		for _, f := range flags {
			flag := &data.EvaluationFlag{
				Key:         f.Key,
				Name:        f.Name,
				Description: f.Description,
				Enabled:     f.Enabled,
				Type:        f.Type,
				CreatedAt:   f.CreatedAt,
				UpdatedAt:   f.UpdatedAt,
			}

			if f.Type == flipt.FlagType_VARIANT_FLAG_TYPE {
				rules, err := srv.store.GetEvaluationRules(ctx, namespaceKey, f.Key)
				if err != nil {
					return nil, fmt.Errorf("getting rules for flag %q: %w", f.Key, err)
				}

				flag.Rules = rules
			}

			if f.Type == flipt.FlagType_BOOLEAN_FLAG_TYPE {
				rollouts, err := srv.store.GetEvaluationRollouts(ctx, namespaceKey, f.Key)
				if err != nil {
					return nil, fmt.Errorf("getting rollout rules for flag %q: %w", f.Key, err)
				}

				flag.Rollouts = rollouts
			}
		}
	}

	return resp, nil
}
