// Copyright (c) 2017 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package vcmock

import (
	"context"
	"os"

	vc "github.com/kata-containers/runtime/virtcontainers"
	"github.com/sirupsen/logrus"
)

// Sandbox is a fake Sandbox type used for testing
type Sandbox struct {
	MockID          string
	MockURL         string
	MockAnnotations map[string]string
	MockContainers  []*Container
	MockNetNs       string
}

// Container is a fake Container type used for testing
type Container struct {
	MockID          string
	MockURL         string
	MockToken       string
	MockProcess     vc.Process
	MockPid         int
	MockSandbox     *Sandbox
	MockAnnotations map[string]string
}

// VCMock is a type that provides an implementation of the VC interface.
// It is used for testing.
type VCMock struct {
	SetLoggerFunc  func(ctx context.Context, logger *logrus.Entry)
	SetFactoryFunc func(ctx context.Context, factory vc.Factory)

	CreateSandboxFunc  func(ctx context.Context, sandboxConfig vc.SandboxConfig) (vc.VCSandbox, error)
	ListSandboxFunc    func(ctx context.Context) ([]vc.SandboxStatus, error)
	FetchSandboxFunc   func(ctx context.Context, sandboxID string) (vc.VCSandbox, error)

	LockSandboxFunc    func(ctx context.Context, sandboxID string) (*os.File, error)
	UnlockSandboxFunc  func(ctx context.Context, lockFile *os.File) error
}
