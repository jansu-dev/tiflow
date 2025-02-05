// Code generated by MockGen. DO NOT EDIT.
// Source: cdc/sinkv2/eventsink/txn/backend.go

// Package mock_txn is a generated GoMock package.
package mock_txn

import (
	context "context"
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"
	eventsink "github.com/pingcap/tiflow/cdc/sinkv2/eventsink"
)

// Mockbackend is a mock of backend interface.
type Mockbackend struct {
	ctrl     *gomock.Controller
	recorder *MockbackendMockRecorder
}

// MockbackendMockRecorder is the mock recorder for Mockbackend.
type MockbackendMockRecorder struct {
	mock *Mockbackend
}

// NewMockbackend creates a new mock instance.
func NewMockbackend(ctrl *gomock.Controller) *Mockbackend {
	mock := &Mockbackend{ctrl: ctrl}
	mock.recorder = &MockbackendMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *Mockbackend) EXPECT() *MockbackendMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *Mockbackend) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockbackendMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*Mockbackend)(nil).Close))
}

// Flush mocks base method.
func (m *Mockbackend) Flush(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Flush", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Flush indicates an expected call of Flush.
func (mr *MockbackendMockRecorder) Flush(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Flush", reflect.TypeOf((*Mockbackend)(nil).Flush), ctx)
}

// MaxFlushInterval mocks base method.
func (m *Mockbackend) MaxFlushInterval() time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MaxFlushInterval")
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// MaxFlushInterval indicates an expected call of MaxFlushInterval.
func (mr *MockbackendMockRecorder) MaxFlushInterval() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MaxFlushInterval", reflect.TypeOf((*Mockbackend)(nil).MaxFlushInterval))
}

// OnTxnEvent mocks base method.
func (m *Mockbackend) OnTxnEvent(e *eventsink.TxnCallbackableEvent) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OnTxnEvent", e)
	ret0, _ := ret[0].(bool)
	return ret0
}

// OnTxnEvent indicates an expected call of OnTxnEvent.
func (mr *MockbackendMockRecorder) OnTxnEvent(e interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OnTxnEvent", reflect.TypeOf((*Mockbackend)(nil).OnTxnEvent), e)
}
