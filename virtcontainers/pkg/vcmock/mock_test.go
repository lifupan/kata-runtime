// Copyright (c) 2017 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package vcmock

import (
	"context"
	"reflect"
	"testing"

	vc "github.com/kata-containers/runtime/virtcontainers"
	"github.com/kata-containers/runtime/virtcontainers/factory"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var (
	loggerTriggered  = 0
	factoryTriggered = 0
)

func TestVCImplementations(t *testing.T) {
	// official implementation
	mainImpl := &vc.VCImpl{}

	// test implementation
	testImpl := &VCMock{}

	var interfaceType vc.VC

	// check that the official implementation implements the
	// interface
	mainImplType := reflect.TypeOf(mainImpl)
	mainImplementsIF := mainImplType.Implements(reflect.TypeOf(&interfaceType).Elem())
	assert.True(t, mainImplementsIF)

	// check that the test implementation implements the
	// interface
	testImplType := reflect.TypeOf(testImpl)
	testImplementsIF := testImplType.Implements(reflect.TypeOf(&interfaceType).Elem())
	assert.True(t, testImplementsIF)
}

func TestVCSandboxImplementations(t *testing.T) {
	// official implementation
	mainImpl := &vc.Sandbox{}

	// test implementation
	testImpl := &Sandbox{}

	var interfaceType vc.VCSandbox

	// check that the official implementation implements the
	// interface
	mainImplType := reflect.TypeOf(mainImpl)
	mainImplementsIF := mainImplType.Implements(reflect.TypeOf(&interfaceType).Elem())
	assert.True(t, mainImplementsIF)

	// check that the test implementation implements the
	// interface
	testImplType := reflect.TypeOf(testImpl)
	testImplementsIF := testImplType.Implements(reflect.TypeOf(&interfaceType).Elem())
	assert.True(t, testImplementsIF)
}

func TestVCContainerImplementations(t *testing.T) {
	// official implementation
	mainImpl := &vc.Container{}

	// test implementation
	testImpl := &Container{}

	var interfaceType vc.VCContainer

	// check that the official implementation implements the
	// interface
	mainImplType := reflect.TypeOf(mainImpl)
	mainImplementsIF := mainImplType.Implements(reflect.TypeOf(&interfaceType).Elem())
	assert.True(t, mainImplementsIF)

	// check that the test implementation implements the
	// interface
	testImplType := reflect.TypeOf(testImpl)
	testImplementsIF := testImplType.Implements(reflect.TypeOf(&interfaceType).Elem())
	assert.True(t, testImplementsIF)
}

func TestVCMockSetLogger(t *testing.T) {
	assert := assert.New(t)

	m := &VCMock{}
	assert.Nil(m.SetLoggerFunc)

	logger := logrus.NewEntry(logrus.New())

	assert.Equal(loggerTriggered, 0)
	ctx := context.Background()
	m.SetLogger(ctx, logger)
	assert.Equal(loggerTriggered, 0)

	m.SetLoggerFunc = func(ctx context.Context, logger *logrus.Entry) {
		loggerTriggered = 1
	}

	m.SetLogger(ctx, logger)
	assert.Equal(loggerTriggered, 1)
}

func TestVCMockCreateSandbox(t *testing.T) {
	assert := assert.New(t)

	m := &VCMock{}
	assert.Nil(m.CreateSandboxFunc)

	ctx := context.Background()
	_, err := m.CreateSandbox(ctx, vc.SandboxConfig{})
	assert.Error(err)
	assert.True(IsMockError(err))

	m.CreateSandboxFunc = func(ctx context.Context, sandboxConfig vc.SandboxConfig) (vc.VCSandbox, error) {
		return &Sandbox{}, nil
	}

	sandbox, err := m.CreateSandbox(ctx, vc.SandboxConfig{})
	assert.NoError(err)
	assert.Equal(sandbox, &Sandbox{})

	// reset
	m.CreateSandboxFunc = nil

	_, err = m.CreateSandbox(ctx, vc.SandboxConfig{})
	assert.Error(err)
	assert.True(IsMockError(err))
}

func TestVCMockListSandbox(t *testing.T) {
	assert := assert.New(t)

	m := &VCMock{}
	assert.Nil(m.ListSandboxFunc)

	ctx := context.Background()
	_, err := m.ListSandbox(ctx)
	assert.Error(err)
	assert.True(IsMockError(err))

	m.ListSandboxFunc = func(ctx context.Context) ([]vc.SandboxStatus, error) {
		return []vc.SandboxStatus{}, nil
	}

	sandboxes, err := m.ListSandbox(ctx)
	assert.NoError(err)
	assert.Equal(sandboxes, []vc.SandboxStatus{})

	// reset
	m.ListSandboxFunc = nil

	_, err = m.ListSandbox(ctx)
	assert.Error(err)
	assert.True(IsMockError(err))
}

func TestVCMockFetchSandbox(t *testing.T) {
	assert := assert.New(t)

	m := &VCMock{}
	config := &vc.SandboxConfig{}
	assert.Nil(m.FetchSandboxFunc)

	ctx := context.Background()
	_, err := m.FetchSandbox(ctx, config.ID)
	assert.Error(err)
	assert.True(IsMockError(err))

	m.FetchSandboxFunc = func(ctx context.Context, id string) (vc.VCSandbox, error) {
		return &Sandbox{}, nil
	}

	sandbox, err := m.FetchSandbox(ctx, config.ID)
	assert.NoError(err)
	assert.Equal(sandbox, &Sandbox{})

	// reset
	m.FetchSandboxFunc = nil

	_, err = m.FetchSandbox(ctx, config.ID)
	assert.Error(err)
	assert.True(IsMockError(err))

}

func TestVCMockSetVMFactory(t *testing.T) {
	assert := assert.New(t)

	m := &VCMock{}
	assert.Nil(m.SetFactoryFunc)

	hyperConfig := vc.HypervisorConfig{
		KernelPath: "foobar",
		ImagePath:  "foobar",
	}
	vmConfig := vc.VMConfig{
		HypervisorType:   vc.MockHypervisor,
		AgentType:        vc.NoopAgentType,
		HypervisorConfig: hyperConfig,
	}

	ctx := context.Background()
	f, err := factory.NewFactory(ctx, factory.Config{VMConfig: vmConfig}, false)
	assert.Nil(err)

	assert.Equal(factoryTriggered, 0)
	m.SetFactory(ctx, f)
	assert.Equal(factoryTriggered, 0)

	m.SetFactoryFunc = func(ctx context.Context, factory vc.Factory) {
		factoryTriggered = 1
	}

	m.SetFactory(ctx, f)
	assert.Equal(factoryTriggered, 1)
}
