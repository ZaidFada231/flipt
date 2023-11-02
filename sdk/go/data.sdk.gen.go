// Code generated by protoc-gen-go-flipt-sdk. DO NOT EDIT.

package sdk

import (
	context "context"
	data "go.flipt.io/flipt/rpc/flipt/data"
)

type Data struct {
	transport     data.DataServiceClient
	tokenProvider ClientTokenProvider
}

func (x *Data) EvaluationSnapshotNamespace(ctx context.Context, v *data.EvaluationNamespaceSnapshotRequest) (*data.EvaluationNamespaceSnapshot, error) {
	ctx, err := authenticate(ctx, x.tokenProvider)
	if err != nil {
		return nil, err
	}
	return x.transport.EvaluationSnapshotNamespace(ctx, v)
}
