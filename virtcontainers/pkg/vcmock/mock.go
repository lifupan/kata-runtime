// Copyright (c) 2017 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

// Description: A mock implementation of virtcontainers that can be used
// for testing.
//
// This implementation calls the function set in the object that
// corresponds to the name of the method (for example, when CreateSandbox()
// is called, that method will try to call CreateSandboxFunc). If no
// function is defined for the method, it will return an error in a
// well-known format. Callers can detect this scenario by calling
// IsMockError().

package vcmock

import (
	"context"
	"fmt"

	vc "github.com/kata-containers/runtime/virtcontainers"
	"github.com/sirupsen/logrus"
	"os"
)

// mockErrorPrefix is a string that all errors returned by the mock
// implementation itself will contain as a prefix.
const mockErrorPrefix = "vcmock forced failure"

// SetLogger implements the VC function of the same name.
func (m *VCMock) SetLogger(ctx context.Context, logger *logrus.Entry) {
	if m.SetLoggerFunc != nil {
		m.SetLoggerFunc(ctx, logger)
	}
}

// SetFactory implements the VC function of the same name.
func (m *VCMock) SetFactory(ctx context.Context, factory vc.Factory) {
	if m.SetFactoryFunc != nil {
		m.SetFactoryFunc(ctx, factory)
	}
}

// CreateSandbox implements the VC function of the same name.
func (m *VCMock) CreateSandbox(ctx context.Context, sandboxConfig vc.SandboxConfig) (vc.VCSandbox, error) {
	if m.CreateSandboxFunc != nil {
		return m.CreateSandboxFunc(ctx, sandboxConfig)
	}

	return nil, fmt.Errorf("%s: %s (%+v): sandboxConfig: %v", mockErrorPrefix, getSelf(), m, sandboxConfig)
}

// FetchSandbox implements the VC function of the same name.
func (m *VCMock) FetchSandbox(ctx context.Context, sandboxID string) (vc.VCSandbox, error) {
	if m.FetchSandboxFunc != nil {
		return m.FetchSandboxFunc(ctx, sandboxID)
	}

	return nil, fmt.Errorf("%s: %s (%+v): sandboxID: %v", mockErrorPrefix, getSelf(), m, sandboxID)
}

// ListSandbox implements the VC function of the same name.
func (m *VCMock) ListSandbox(ctx context.Context) ([]vc.SandboxStatus, error) {
	if m.ListSandboxFunc != nil {
		return m.ListSandboxFunc(ctx)
	}

	return nil, fmt.Errorf("%s: %s", mockErrorPrefix, getSelf())
}

// ListSandbox implements the VC function of the same name.
func (m *VCMock) LockSandbox(ctx context.Context, sandboxID string) (*os.File, error) {
	if m.LockSandboxFunc != nil {
		return m.LockSandboxFunc(ctx, sandboxID)
	}

	return nil, fmt.Errorf("%s: %s", mockErrorPrefix, getSelf())
}

// ListSandbox implements the VC function of the same name.
func (m *VCMock) UnlockSandbox(ctx context.Context, fileLock *os.File) error {
	if m.UnlockSandboxFunc != nil {
		return m.UnlockSandboxFunc(ctx, fileLock)
	}

	return fmt.Errorf("%s: %s", mockErrorPrefix, getSelf())
}