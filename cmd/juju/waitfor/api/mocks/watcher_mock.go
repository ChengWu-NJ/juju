// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/cmd/juju/waitfor/api (interfaces: WatchAllAPI,AllWatcher)

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	api "github.com/juju/juju/cmd/juju/waitfor/api"
	params "github.com/juju/juju/rpc/params"
)

// MockWatchAllAPI is a mock of WatchAllAPI interface.
type MockWatchAllAPI struct {
	ctrl     *gomock.Controller
	recorder *MockWatchAllAPIMockRecorder
}

// MockWatchAllAPIMockRecorder is the mock recorder for MockWatchAllAPI.
type MockWatchAllAPIMockRecorder struct {
	mock *MockWatchAllAPI
}

// NewMockWatchAllAPI creates a new mock instance.
func NewMockWatchAllAPI(ctrl *gomock.Controller) *MockWatchAllAPI {
	mock := &MockWatchAllAPI{ctrl: ctrl}
	mock.recorder = &MockWatchAllAPIMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockWatchAllAPI) EXPECT() *MockWatchAllAPIMockRecorder {
	return m.recorder
}

// WatchAll mocks base method.
func (m *MockWatchAllAPI) WatchAll() (api.AllWatcher, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WatchAll")
	ret0, _ := ret[0].(api.AllWatcher)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WatchAll indicates an expected call of WatchAll.
func (mr *MockWatchAllAPIMockRecorder) WatchAll() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WatchAll", reflect.TypeOf((*MockWatchAllAPI)(nil).WatchAll))
}

// MockAllWatcher is a mock of AllWatcher interface.
type MockAllWatcher struct {
	ctrl     *gomock.Controller
	recorder *MockAllWatcherMockRecorder
}

// MockAllWatcherMockRecorder is the mock recorder for MockAllWatcher.
type MockAllWatcherMockRecorder struct {
	mock *MockAllWatcher
}

// NewMockAllWatcher creates a new mock instance.
func NewMockAllWatcher(ctrl *gomock.Controller) *MockAllWatcher {
	mock := &MockAllWatcher{ctrl: ctrl}
	mock.recorder = &MockAllWatcherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAllWatcher) EXPECT() *MockAllWatcherMockRecorder {
	return m.recorder
}

// Next mocks base method.
func (m *MockAllWatcher) Next() ([]params.Delta, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Next")
	ret0, _ := ret[0].([]params.Delta)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Next indicates an expected call of Next.
func (mr *MockAllWatcherMockRecorder) Next() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Next", reflect.TypeOf((*MockAllWatcher)(nil).Next))
}

// Stop mocks base method.
func (m *MockAllWatcher) Stop() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop")
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop.
func (mr *MockAllWatcherMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockAllWatcher)(nil).Stop))
}
