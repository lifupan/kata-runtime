// Copyright (c) 2017 Intel Corporation
// Copyright (c) 2018 HyperHQ Inc.
//
// SPDX-License-Identifier: Apache-2.0
//

package kata

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/containerd/containerd/namespaces"
	taskAPI "github.com/containerd/containerd/runtime/v2/task"
	vc "github.com/kata-containers/runtime/virtcontainers"
	"github.com/kata-containers/runtime/virtcontainers/pkg/vcmock"
	"github.com/stretchr/testify/assert"
)

func TestPauseContainerSuccess(t *testing.T) {
	assert := assert.New(t)

	sandbox := &vcmock.Sandbox{
		MockID: testSandboxID,
	}

	path, err := createTempContainerIDMapping(testContainerID, testSandboxID)
	assert.NoError(err)
	defer os.RemoveAll(path)

	testingImpl.PauseContainerFunc = func(sandboxID, containerID string) error {
		return nil
	}
	defer func() {
		testingImpl.PauseContainerFunc = nil
	}()

	testingImpl.StatusContainerFunc = func(sandboxID, containerID string) (vc.ContainerStatus, error) {
		return vc.ContainerStatus{
			ID:          testContainerID,
			Annotations: make(map[string]string),
			State: vc.State{
				State: vc.StateRunning,
			},
		}, nil
	}
	defer func() {
		testingImpl.StatusContainerFunc = nil
	}()

	s := &service{
		id:         testSandboxID,
		sandbox:    sandbox,
		containers: make(map[string]*container),
		processes:  make(map[uint32]string),
	}

	reqCreate := &taskAPI.CreateTaskRequest{
		ID: testContainerID,
	}
	s.containers[testContainerID], err = newContainer(s, reqCreate, TestPid)
	assert.NoError(err)

	reqPause := &taskAPI.PauseRequest{
		ID: testContainerID,
	}
	ctx := namespaces.WithNamespace(context.Background(), "UnitTest")

	_, err = s.Pause(ctx, reqPause)
	assert.NoError(err)
}

func TestPauseContainerFail(t *testing.T) {
	assert := assert.New(t)

	sandbox := &vcmock.Sandbox{
		MockID: testSandboxID,
	}

	path, err := ioutil.TempDir("", "containers-mapping")
	assert.NoError(err)
	defer os.RemoveAll(path)
	ctrsMapTreePath = path

	testingImpl.PauseContainerFunc = func(sandboxID, containerID string) error {
		return nil
	}
	defer func() {
		testingImpl.PauseContainerFunc = nil
	}()

	testingImpl.StatusContainerFunc = func(sandboxID, containerID string) (vc.ContainerStatus, error) {
		return vc.ContainerStatus{
			ID:          testContainerID,
			Annotations: make(map[string]string),
			State: vc.State{
				State: vc.StateRunning,
			},
		}, nil
	}
	defer func() {
		testingImpl.StatusContainerFunc = nil
	}()

	s := &service{
		id:         testSandboxID,
		sandbox:    sandbox,
		containers: make(map[string]*container),
		processes:  make(map[uint32]string),
	}

	reqPause := &taskAPI.PauseRequest{
		ID: testContainerID,
	}
	ctx := namespaces.WithNamespace(context.Background(), "UnitTest")

	_, err = s.Pause(ctx, reqPause)
	assert.Error(err)
}

func TestResumeContainerSuccess(t *testing.T) {
	assert := assert.New(t)

	sandbox := &vcmock.Sandbox{
		MockID: testSandboxID,
	}

	path, err := createTempContainerIDMapping(testContainerID, testSandboxID)
	assert.NoError(err)
	defer os.RemoveAll(path)

	testingImpl.ResumeContainerFunc = func(sandboxID, containerID string) error {
		return nil
	}
	defer func() {
		testingImpl.ResumeContainerFunc = nil
	}()

	testingImpl.StatusContainerFunc = func(sandboxID, containerID string) (vc.ContainerStatus, error) {
		return vc.ContainerStatus{
			ID:          testContainerID,
			Annotations: make(map[string]string),
			State: vc.State{
				State: vc.StateRunning,
			},
		}, nil
	}

	defer func() {
		testingImpl.StatusContainerFunc = nil
	}()

	s := &service{
		id:         testSandboxID,
		sandbox:    sandbox,
		containers: make(map[string]*container),
		processes:  make(map[uint32]string),
	}

	reqCreate := &taskAPI.CreateTaskRequest{
		ID: testContainerID,
	}
	s.containers[testContainerID], err = newContainer(s, reqCreate, TestPid)
	assert.NoError(err)

	reqResume := &taskAPI.ResumeRequest{
		ID: testContainerID,
	}
	ctx := namespaces.WithNamespace(context.Background(), "UnitTest")

	_, err = s.Resume(ctx, reqResume)
	assert.NoError(err)
}

func TestResumeContainerFail(t *testing.T) {
	assert := assert.New(t)

	sandbox := &vcmock.Sandbox{
		MockID: testSandboxID,
	}

	path, err := ioutil.TempDir("", "containers-mapping")
	assert.NoError(err)
	defer os.RemoveAll(path)
	ctrsMapTreePath = path

	testingImpl.ResumeContainerFunc = func(sandboxID, containerID string) error {
		return nil
	}
	defer func() {
		testingImpl.ResumeContainerFunc = nil
	}()
	testingImpl.StatusContainerFunc = func(sandboxID, containerID string) (vc.ContainerStatus, error) {
		return vc.ContainerStatus{
			ID:          testContainerID,
			Annotations: make(map[string]string),
			State: vc.State{
				State: vc.StateRunning,
			},
		}, nil
	}
	defer func() {
		testingImpl.StatusContainerFunc = nil
	}()

	s := &service{
		id:         testSandboxID,
		sandbox:    sandbox,
		containers: make(map[string]*container),
		processes:  make(map[uint32]string),
	}

	reqResume := &taskAPI.ResumeRequest{
		ID: testContainerID,
	}
	ctx := namespaces.WithNamespace(context.Background(), "UnitTest")

	_, err = s.Resume(ctx, reqResume)
	assert.Error(err)
}
