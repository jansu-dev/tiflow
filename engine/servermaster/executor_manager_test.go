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

package servermaster

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	pb "github.com/pingcap/tiflow/engine/enginepb"
	"github.com/pingcap/tiflow/engine/model"
	"github.com/pingcap/tiflow/engine/servermaster/executormeta"
	execModel "github.com/pingcap/tiflow/engine/servermaster/executormeta/model"
	"github.com/stretchr/testify/require"
)

func TestExecutorManager(t *testing.T) {
	t.Parallel()

	metaClient := executormeta.NewMockClient(gomock.NewController(t))

	ctx, cancel := context.WithCancel(context.Background())
	heartbeatTTL := time.Millisecond * 100
	checkInterval := time.Millisecond * 10
	mgr := NewExecutorManagerImpl(metaClient, heartbeatTTL, checkInterval, nil)

	// register an executor server
	executorAddr := "127.0.0.1:10001"
	registerReq := &pb.RegisterExecutorRequest{
		Executor: &pb.Executor{
			Address:    executorAddr,
			Capability: 2,
		},
	}
	metaClient.EXPECT().
		CreateExecutor(gomock.Any(), gomock.Any()).Times(1).
		DoAndReturn(func(ctx context.Context, executor *execModel.Executor) error {
			require.NotEmpty(t, executor.ID)
			require.Equal(t, executorAddr, executor.Address)
			require.Equal(t, 2, executor.Capability)
			return nil
		})

	executor, err := mgr.AllocateNewExec(ctx, registerReq)
	require.Nil(t, err)

	addr, ok := mgr.GetAddr(executor.ID)
	require.True(t, ok)
	require.Equal(t, "127.0.0.1:10001", addr)

	require.Equal(t, 1, mgr.ExecutorCount(model.Initing))
	require.Equal(t, 0, mgr.ExecutorCount(model.Running))
	mgr.mu.Lock()
	require.Contains(t, mgr.executors, executor.ID)
	mgr.mu.Unlock()

	newHeartbeatReq := func() *pb.HeartbeatRequest {
		return &pb.HeartbeatRequest{
			ExecutorId: string(executor.ID),
			Status:     int32(model.Running),
			Timestamp:  uint64(time.Now().Unix()),
			Ttl:        uint64(10), // 10ms ttl
		}
	}

	// test executor heartbeat
	resp, err := mgr.HandleHeartbeat(newHeartbeatReq())
	require.Nil(t, err)
	require.Nil(t, resp.Err)

	metaClient.EXPECT().QueryExecutors(gomock.Any()).Times(1).Return([]*execModel.Executor{}, nil)
	metaClient.EXPECT().DeleteExecutor(gomock.Any(), executor.ID).Times(1).Return(nil)

	mgr.Start(ctx)

	require.Eventually(t, func() bool {
		return mgr.ExecutorCount(model.Running) == 0
	}, time.Second*2, time.Millisecond*50)

	// test late heartbeat request after executor is offline
	resp, err = mgr.HandleHeartbeat(newHeartbeatReq())
	require.Nil(t, err)
	require.NotNil(t, resp.Err)
	require.Equal(t, pb.ErrorCode_UnknownExecutor, resp.Err.GetCode())

	cancel()
	mgr.Stop()
}

func TestExecutorManagerWatch(t *testing.T) {
	t.Parallel()

	metaClient := executormeta.NewMockClient(gomock.NewController(t))

	heartbeatTTL := time.Millisecond * 400
	checkInterval := time.Millisecond * 50
	ctx, cancel := context.WithCancel(context.Background())
	mgr := NewExecutorManagerImpl(metaClient, heartbeatTTL, checkInterval, nil)

	// register an executor server
	executorAddr := "127.0.0.1:10001"
	registerReq := &pb.RegisterExecutorRequest{
		Executor: &pb.Executor{
			Address:    executorAddr,
			Capability: 2,
		},
	}
	metaClient.EXPECT().
		CreateExecutor(gomock.Any(), gomock.Any()).Times(1).
		DoAndReturn(func(ctx context.Context, executor *execModel.Executor) error {
			require.NotEmpty(t, executor.ID)
			require.Equal(t, executorAddr, executor.Address)
			require.Equal(t, 2, executor.Capability)
			return nil
		})
	executor, err := mgr.AllocateNewExec(ctx, registerReq)
	require.Nil(t, err)

	executorID1 := executor.ID
	snap, stream, err := mgr.WatchExecutors(context.Background())
	require.NoError(t, err)
	require.Equal(t, map[model.ExecutorID]string{
		executorID1: executor.Address,
	}, snap)

	// register another executor server
	executorAddr = "127.0.0.1:10002"
	registerReq = &pb.RegisterExecutorRequest{
		Executor: &pb.Executor{
			Address:    executorAddr,
			Capability: 2,
		},
	}
	metaClient.EXPECT().
		CreateExecutor(gomock.Any(), gomock.Any()).Times(1).
		DoAndReturn(func(ctx context.Context, executor *execModel.Executor) error {
			require.NotEmpty(t, executor.ID)
			require.Equal(t, executorAddr, executor.Address)
			require.Equal(t, 2, executor.Capability)
			return nil
		})
	executor, err = mgr.AllocateNewExec(ctx, registerReq)
	require.Nil(t, err)

	executorID2 := executor.ID
	event := <-stream.C
	require.Equal(t, model.ExecutorStatusChange{
		ID:   executorID2,
		Tp:   model.EventExecutorOnline,
		Addr: "127.0.0.1:10002",
	}, event)

	newHeartbeatReq := func(executorID model.ExecutorID) *pb.HeartbeatRequest {
		return &pb.HeartbeatRequest{
			ExecutorId: string(executorID),
			Status:     int32(model.Running),
			Timestamp:  uint64(time.Now().Unix()),
			Ttl:        uint64(100), // 10ms ttl
		}
	}

	bgExecutorHeartbeat := func(
		ctx context.Context, wg *sync.WaitGroup, executorID model.DeployNodeID,
	) context.CancelFunc {
		// send a synchronous heartbeat first in order to ensure the online
		// count of this executor takes effect immediately.
		resp, err := mgr.HandleHeartbeat(newHeartbeatReq(executorID))
		require.NoError(t, err)
		require.Nil(t, resp.Err)

		ctxIn, cancelIn := context.WithCancel(ctx)
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctxIn.Done():
					return
				case <-ticker.C:
					resp, err := mgr.HandleHeartbeat(newHeartbeatReq(executorID))
					require.NoError(t, err)
					require.Nil(t, resp.Err)
				}
			}
		}()
		return cancelIn
	}

	metaClient.EXPECT().QueryExecutors(gomock.Any()).Times(1).
		Return([]*execModel.Executor{
			{ID: executorID1, Address: "127.0.0.1:10001"},
			{ID: executorID2, Address: "127.0.0.1:10002"},
		}, nil)
	metaClient.EXPECT().DeleteExecutor(gomock.Any(), executorID1).Times(1).Return(nil)
	metaClient.EXPECT().DeleteExecutor(gomock.Any(), executorID2).Times(1).Return(nil)

	mgr.Start(ctx)

	// mgr.Start will reset executors first, so there will be two online events.
	event = <-stream.C
	require.Equal(t, model.ExecutorStatusChange{
		ID:   executorID1,
		Tp:   model.EventExecutorOnline,
		Addr: "127.0.0.1:10001",
	}, event)
	event = <-stream.C
	require.Equal(t, model.ExecutorStatusChange{
		ID:   executorID2,
		Tp:   model.EventExecutorOnline,
		Addr: "127.0.0.1:10002",
	}, event)

	require.Equal(t, 0, mgr.ExecutorCount(model.Running))
	var wg sync.WaitGroup
	cancel1 := bgExecutorHeartbeat(ctx, &wg, executorID1)
	cancel2 := bgExecutorHeartbeat(ctx, &wg, executorID2)
	require.Equal(t, 2, mgr.ExecutorCount(model.Running))

	// executor-1 will time out
	cancel1()
	require.Eventually(t, func() bool {
		return mgr.ExecutorCount(model.Running) == 1
	}, time.Second*2, time.Millisecond*5)

	event = <-stream.C
	require.Equal(t, model.ExecutorStatusChange{
		ID:   executorID1,
		Tp:   model.EventExecutorOffline,
		Addr: "127.0.0.1:10001",
	}, event)

	// executor-2 will time out
	cancel2()
	wg.Wait()
	event = <-stream.C
	require.Equal(t, model.ExecutorStatusChange{
		ID:   executorID2,
		Tp:   model.EventExecutorOffline,
		Addr: "127.0.0.1:10002",
	}, event)

	cancel()
	mgr.Stop()
}
