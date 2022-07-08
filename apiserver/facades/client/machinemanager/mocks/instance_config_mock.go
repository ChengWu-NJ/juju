// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/apiserver/facades/client/machinemanager (interfaces: ControllerBackend)

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	controller "github.com/juju/juju/controller"
	network "github.com/juju/juju/core/network"
	names "github.com/juju/names/v4"
)

// MockControllerBackend is a mock of ControllerBackend interface.
type MockControllerBackend struct {
	ctrl     *gomock.Controller
	recorder *MockControllerBackendMockRecorder
}

// MockControllerBackendMockRecorder is the mock recorder for MockControllerBackend.
type MockControllerBackendMockRecorder struct {
	mock *MockControllerBackend
}

// NewMockControllerBackend creates a new mock instance.
func NewMockControllerBackend(ctrl *gomock.Controller) *MockControllerBackend {
	mock := &MockControllerBackend{ctrl: ctrl}
	mock.recorder = &MockControllerBackendMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockControllerBackend) EXPECT() *MockControllerBackendMockRecorder {
	return m.recorder
}

// APIHostPortsForAgents mocks base method.
func (m *MockControllerBackend) APIHostPortsForAgents() ([]network.SpaceHostPorts, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "APIHostPortsForAgents")
	ret0, _ := ret[0].([]network.SpaceHostPorts)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// APIHostPortsForAgents indicates an expected call of APIHostPortsForAgents.
func (mr *MockControllerBackendMockRecorder) APIHostPortsForAgents() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "APIHostPortsForAgents", reflect.TypeOf((*MockControllerBackend)(nil).APIHostPortsForAgents))
}

// ControllerConfig mocks base method.
func (m *MockControllerBackend) ControllerConfig() (controller.Config, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ControllerConfig")
	ret0, _ := ret[0].(controller.Config)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ControllerConfig indicates an expected call of ControllerConfig.
func (mr *MockControllerBackendMockRecorder) ControllerConfig() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ControllerConfig", reflect.TypeOf((*MockControllerBackend)(nil).ControllerConfig))
}

// ControllerTag mocks base method.
func (m *MockControllerBackend) ControllerTag() names.ControllerTag {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ControllerTag")
	ret0, _ := ret[0].(names.ControllerTag)
	return ret0
}

// ControllerTag indicates an expected call of ControllerTag.
func (mr *MockControllerBackendMockRecorder) ControllerTag() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ControllerTag", reflect.TypeOf((*MockControllerBackend)(nil).ControllerTag))
}