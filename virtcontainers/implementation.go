// Copyright (c) 2017 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

// Description: The true virtcontainers function of the same name.
// This indirection is required to allow an alternative implemenation to be
// used for testing purposes.

package virtcontainers

import (
	"context"
	"github.com/sirupsen/logrus"
	"os"
)

// VCImpl is the official virtcontainers function of the same name.
type VCImpl struct {
	factory Factory
}

// SetLogger implements the VC function of the same name.
func (impl *VCImpl) SetLogger(ctx context.Context, logger *logrus.Entry) {
	SetLogger(ctx, logger)
}

// SetFactory implements the VC function of the same name.
func (impl *VCImpl) SetFactory(ctx context.Context, factory Factory) {
	impl.factory = factory
}

// CreateSandbox implements the VC function of the same name.
func (impl *VCImpl) CreateSandbox(ctx context.Context, sandboxConfig SandboxConfig) (VCSandbox, error) {
	return CreateSandbox(ctx, sandboxConfig, impl.factory)
}

// ListSandbox implements the VC function of the same name.
func (impl *VCImpl) ListSandbox(ctx context.Context) ([]SandboxStatus, error) {
	return ListSandbox(ctx)
}

// FetchSandbox will find out and connect to an existing sandbox and
// return the sandbox structure.
func (impl *VCImpl) FetchSandbox(ctx context.Context, sandboxID string) (VCSandbox, error) {
	return FetchSandbox(ctx, sandboxID)
}

// StatusSandbox implements the VC function of the same name.
func (impl *VCImpl) StatusSandbox(ctx context.Context, sandboxID string) (SandboxStatus, error) {
	return StatusSandbox(ctx, sandboxID)
}

// LockSandbox implements the VC function of the same name.
func (impl *VCImpl) LockSandbox(ctx context.Context, sandboxID string) (*os.File, error) {
	return LockSandbox(ctx, sandboxID)
}

// UnlockSandbox implements the VC function of the same name.
func (impl *VCImpl) UnlockSandbox(ctx context.Context, lockFile *os.File) error {
	return UnlockSandbox(ctx, lockFile)
}