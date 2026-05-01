/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mocks

import (
	"context"
	"reflect"

	"github.com/google/go-github/v74/github"
	"go.uber.org/mock/gomock"
)

// MockGitHubClient is a mock of controller.GitHubClient interface.
type MockGitHubClient struct {
	ctrl     *gomock.Controller
	recorder *MockGitHubClientMockRecorder
}

// MockGitHubClientMockRecorder is the mock recorder for MockGitHubClient.
type MockGitHubClientMockRecorder struct {
	mock *MockGitHubClient
}

// NewMockGitHubClient creates a new mock instance.
func NewMockGitHubClient(ctrl *gomock.Controller) *MockGitHubClient {
	mock := &MockGitHubClient{ctrl: ctrl}
	mock.recorder = &MockGitHubClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockGitHubClient) EXPECT() *MockGitHubClientMockRecorder {
	return m.recorder
}

// CreateKey mocks base method.
func (m *MockGitHubClient) CreateKey(ctx context.Context, owner, repo string, key *github.Key) (*github.Key, *github.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateKey", ctx, owner, repo, key)
	ret0, _ := ret[0].(*github.Key)
	ret1, _ := ret[1].(*github.Response)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CreateKey indicates an expected call of CreateKey.
func (mr *MockGitHubClientMockRecorder) CreateKey(ctx, owner, repo, key interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateKey", reflect.TypeOf((*MockGitHubClient)(nil).CreateKey), ctx, owner, repo, key)
}

// DeleteKey mocks base method.
func (m *MockGitHubClient) DeleteKey(ctx context.Context, owner, repo string, id int64) (*github.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteKey", ctx, owner, repo, id)
	ret0, _ := ret[0].(*github.Response)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteKey indicates an expected call of DeleteKey.
func (mr *MockGitHubClientMockRecorder) DeleteKey(ctx, owner, repo, id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteKey", reflect.TypeOf((*MockGitHubClient)(nil).DeleteKey), ctx, owner, repo, id)
}
