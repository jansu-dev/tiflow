// Copyright 2022 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package manager

import (
	"context"
	"fmt"
	"testing"

	pb "github.com/pingcap/tiflow/engine/enginepb"
	resModel "github.com/pingcap/tiflow/engine/pkg/externalresource/resourcemeta/model"
	pkgOrm "github.com/pingcap/tiflow/engine/pkg/orm"
	"github.com/pingcap/tiflow/engine/pkg/rpcerror"
	"github.com/pingcap/tiflow/engine/pkg/rpcutil"
	"github.com/pingcap/tiflow/engine/pkg/tenant"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"golang.org/x/time/rate"
	"google.golang.org/grpc/codes"
)

var _ pb.ResourceManagerServer = (*Service)(nil)

type serviceTestSuite struct {
	service              *Service
	executorInfoProvider *MockExecutorInfoProvider
	meta                 pkgOrm.Client
}

var serviceMockData = []*resModel.ResourceMeta{
	{
		ID:       "/local/test/1",
		Job:      "test-job-1",
		Worker:   "test-worker-1",
		Executor: "executor-1",
		Deleted:  false,
	},
	{
		ID:       "/local/test/2",
		Job:      "test-job-1",
		Worker:   "test-worker-1",
		Executor: "executor-1",
		Deleted:  false,
	},
	{
		ID:       "/local/test/3",
		Job:      "test-job-1",
		Worker:   "test-worker-2",
		Executor: "executor-2",
		Deleted:  false,
	},
	{
		ID:       "/local/test/4",
		Job:      "test-job-1",
		Worker:   "test-worker-2",
		Executor: "executor-2",
		Deleted:  false,
	},
	{
		ID:       "/local/test/5",
		Job:      "test-job-1",
		Worker:   "test-worker-3",
		Executor: "executor-4",
		Deleted:  true,
	},
}

func newServiceTestSuite(t *testing.T) *serviceTestSuite {
	execPro := NewMockExecutorInfoProvider()
	meta, err := pkgOrm.NewMockClient()
	require.NoError(t, err)
	id := "leader"
	leaderVal := &atomic.Value{}
	leaderVal.Store(&rpcutil.Member{Name: id})
	srvc := NewService(meta, execPro, rpcutil.NewPreRPCHook[pb.ResourceManagerClient](
		id,
		leaderVal,
		&rpcutil.LeaderClientWithLock[pb.ResourceManagerClient]{},
		atomic.NewBool(true),
		&rate.Limiter{}, nil))
	return &serviceTestSuite{
		service:              srvc,
		executorInfoProvider: execPro,
		meta:                 meta,
	}
}

func (s *serviceTestSuite) LoadMockData() {
	for _, resource := range serviceMockData {
		_ = s.meta.UpsertResource(context.Background(), resource)
	}

	for i := 1; i <= 4; i++ {
		s.executorInfoProvider.AddExecutor(
			fmt.Sprintf("executor-%d", i),
			fmt.Sprintf("addr-%d", i))
	}
}

func TestServiceBasics(t *testing.T) {
	fakeProjectInfo := tenant.NewProjectInfo("fakeTenant", "fakeProject")
	suite := newServiceTestSuite(t)
	suite.LoadMockData()

	ctx := context.Background()
	_, err := suite.service.CreateResource(ctx, &pb.CreateResourceRequest{
		ProjectInfo:     &pb.ProjectInfo{TenantId: fakeProjectInfo.TenantID(), ProjectId: fakeProjectInfo.ProjectID()},
		ResourceId:      "/local/test/6",
		CreatorExecutor: "executor-1",
		JobId:           "test-job-1",
		CreatorWorkerId: "test-worker-4",
	})
	require.NoError(t, err)

	_, err = suite.service.CreateResource(ctx, &pb.CreateResourceRequest{
		ProjectInfo:     &pb.ProjectInfo{TenantId: fakeProjectInfo.TenantID(), ProjectId: fakeProjectInfo.ProjectID()},
		ResourceId:      "/local/test/6",
		CreatorExecutor: "executor-1",
		JobId:           "test-job-1",
		CreatorWorkerId: "test-worker-4",
	})
	require.Error(t, err)
	code, ok := rpcerror.GRPCStatusCode(err)
	require.True(t, ok)
	require.Equal(t, codes.AlreadyExists, code)

	execID, ok, err := suite.service.GetPlacementConstraint(ctx,
		resModel.ResourceKey{
			JobID: "test-job-1",
			ID:    "/local/test/6",
		})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "executor-1", string(execID))

	execID, ok, err = suite.service.GetPlacementConstraint(ctx,
		resModel.ResourceKey{
			JobID: "test-job-1",
			ID:    "/local/test/1",
		})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "executor-1", string(execID))

	execID, ok, err = suite.service.GetPlacementConstraint(ctx,
		resModel.ResourceKey{
			JobID: "test-job-1",
			ID:    "/local/test/2",
		})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "executor-1", string(execID))

	resp, err := suite.service.QueryResource(ctx, &pb.QueryResourceRequest{
		ResourceKey: &pb.ResourceKey{
			JobId: "test-job-1", ResourceId: "/local/test/2",
		},
	})
	require.NoError(t, err)
	require.Equal(t, &pb.QueryResourceResponse{
		CreatorExecutor: "executor-1",
		JobId:           "test-job-1",
		CreatorWorkerId: "test-worker-1",
	}, resp)

	_, err = suite.service.RemoveResource(ctx, &pb.RemoveResourceRequest{
		ResourceKey: &pb.ResourceKey{
			JobId: "test-job-1", ResourceId: "/local/test/1",
		},
	})
	require.NoError(t, err)

	_, err = suite.service.RemoveResource(ctx, &pb.RemoveResourceRequest{
		ResourceKey: &pb.ResourceKey{
			JobId: "test-job-1", ResourceId: "/local/test/2",
		},
	})
	require.NoError(t, err)

	_, err = suite.service.RemoveResource(ctx, &pb.RemoveResourceRequest{
		ResourceKey: &pb.ResourceKey{
			JobId: "test-job-1", ResourceId: "/local/test/6",
		},
	})
	require.NoError(t, err)

	_, _, err = suite.service.GetPlacementConstraint(ctx,
		resModel.ResourceKey{
			JobID: "test-job-1",
			ID:    "/local/test/2",
		})
	require.Error(t, err)
	require.Regexp(t, ".*ErrResourceDoesNotExist.*", err)

	_, err = suite.service.QueryResource(ctx, &pb.QueryResourceRequest{
		ResourceKey: &pb.ResourceKey{
			JobId: "test-job-1", ResourceId: "/local/test/2",
		},
	})
	code, ok = rpcerror.GRPCStatusCode(err)
	require.True(t, ok)
	require.Equal(t, codes.NotFound, code)

	_, err = suite.service.QueryResource(ctx, &pb.QueryResourceRequest{
		ResourceKey: &pb.ResourceKey{
			JobId: "test-job-1", ResourceId: "/local/test/non-existent",
		},
	})
	code, ok = rpcerror.GRPCStatusCode(err)
	require.True(t, ok)
	require.Equal(t, codes.NotFound, code)
}

func TestServiceResourceTypeNoConstraint(t *testing.T) {
	suite := newServiceTestSuite(t)
	suite.LoadMockData()

	_, ok, err := suite.service.GetPlacementConstraint(context.Background(),
		resModel.ResourceKey{
			JobID: "test-job-1",
			ID:    "/s3/fake-s3-resource",
		})
	require.NoError(t, err)
	require.False(t, ok)
}
