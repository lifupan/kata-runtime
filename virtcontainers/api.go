// Copyright (c) 2016 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package virtcontainers

import (
	"context"
	"os"
	"runtime"
	"syscall"

	"fmt"
	deviceApi "github.com/kata-containers/runtime/virtcontainers/device/api"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"
)

func init() {
	runtime.LockOSThread()
}

var virtLog = logrus.WithField("source", "virtcontainers")

// trace creates a new tracing span based on the specified name and parent
// context.
func trace(parent context.Context, name string) (opentracing.Span, context.Context) {
	span, ctx := opentracing.StartSpanFromContext(parent, name)

	// Should not need to be changed (again).
	span.SetTag("source", "virtcontainers")
	span.SetTag("component", "virtcontainers")

	// Should be reset as new subsystems are entered.
	span.SetTag("subsystem", "api")

	return span, ctx
}

// SetLogger sets the logger for virtcontainers package.
func SetLogger(ctx context.Context, logger *logrus.Entry) {
	fields := virtLog.Data
	virtLog = logger.WithFields(fields)

	deviceApi.SetLogger(virtLog)
}

// CreateSandbox is the virtcontainers sandbox creation entry point.
// CreateSandbox creates a sandbox and its containers. It does not start them.
func CreateSandbox(ctx context.Context, sandboxConfig SandboxConfig, factory Factory) (VCSandbox, error) {
	span, ctx := trace(ctx, "CreateSandbox")
	defer span.Finish()

	s, err := createSandboxFromConfig(ctx, sandboxConfig, factory)
	if err == nil {
		s.Release()
	}

	return s, err
}

func createSandboxFromConfig(ctx context.Context, sandboxConfig SandboxConfig, factory Factory) (*Sandbox, error) {
	span, ctx := trace(ctx, "createSandboxFromConfig")
	defer span.Finish()

	var err error

	// Create the sandbox.
	s, err := createSandbox(ctx, sandboxConfig, factory)
	if err != nil {
		return nil, err
	}

	// cleanup sandbox resources in case of any failure
	defer func() {
		if err != nil {
			s.Delete()
		}
	}()

	// Create the sandbox network
	if err = s.createNetwork(); err != nil {
		return nil, err
	}

	// network rollback
	defer func() {
		if err != nil && s.networkNS.NetNsCreated {
			s.removeNetwork()
		}
	}()

	// Start the VM
	if err = s.startVM(); err != nil {
		return nil, err
	}

	// rollback to stop VM if error occurs
	defer func() {
		if err != nil {
			s.stopVM()
		}
	}()

	// Once startVM is done, we want to guarantee
	// that the sandbox is manageable. For that we need
	// to start the sandbox inside the VM.
	if err = s.agent.startSandbox(s); err != nil {
		return nil, err
	}

	// rollback to stop sandbox in VM
	defer func() {
		if err != nil {
			s.agent.stopSandbox(s)
		}
	}()

	if err = s.getAndStoreGuestDetails(); err != nil {
		return nil, err
	}

	// Create Containers
	if err = s.createContainers(); err != nil {
		return nil, err
	}

	// The sandbox is completely created now, we can store it.
	if err = s.storeSandbox(); err != nil {
		return nil, err
	}

	// Setup host cgroups
	if err = s.setupCgroups(); err != nil {
		return nil, err
	}

	return s, nil
}

// FetchSandbox is the virtcontainers sandbox fetching entry point.
// FetchSandbox will find out and connect to an existing sandbox and
// return the sandbox structure. The caller is responsible of calling
// VCSandbox.Release() after done with it.
func FetchSandbox(ctx context.Context, sandboxID string) (VCSandbox, error) {
	span, ctx := trace(ctx, "FetchSandbox")
	defer span.Finish()

	if sandboxID == "" {
		return nil, errNeedSandboxID
	}

	lockFile, err := LockSandbox(ctx, sandboxID)
	if err != nil {
		return nil, err
	}
	defer UnlockSandbox(ctx, lockFile)

	// Fetch the sandbox from storage and create it.
	s, err := fetchSandbox(ctx, sandboxID)
	if err != nil {
		return nil, err
	}

	// If the proxy is KataBuiltInProxyType type, it needs to restart the proxy to watch the
	// guest console if it hadn't been watched.
	if isProxyBuiltIn(s.config.ProxyType) {
		err = s.startProxy()
		if err != nil {
			s.Release()
			return nil, err
		}
	}

	return s, nil
}

// ListSandbox is the virtcontainers sandbox listing entry point.
func ListSandbox(ctx context.Context) ([]SandboxStatus, error) {
	span, ctx := trace(ctx, "ListSandbox")
	defer span.Finish()

	dir, err := os.Open(configStoragePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No sandbox directory is not an error
			return []SandboxStatus{}, nil
		}
		return []SandboxStatus{}, err
	}

	defer dir.Close()

	sandboxesID, err := dir.Readdirnames(0)
	if err != nil {
		return []SandboxStatus{}, err
	}

	var sandboxStatusList []SandboxStatus

	for _, sandboxID := range sandboxesID {
		sandboxStatus, err := StatusSandbox(ctx, sandboxID)
		if err != nil {
			continue
		}

		sandboxStatusList = append(sandboxStatusList, sandboxStatus)
	}

	return sandboxStatusList, nil
}

// StatusSandbox is the virtcontainers sandbox status entry point.
func StatusSandbox(ctx context.Context, sandboxID string) (SandboxStatus, error) {
	span, ctx := trace(ctx, "StatusSandbox")
	defer span.Finish()

	if sandboxID == "" {
		return SandboxStatus{}, errNeedSandboxID
	}

	lockFile, err := LockSandbox(ctx, sandboxID)
	if err != nil {
		return SandboxStatus{}, err
	}
	defer UnlockSandbox(ctx, lockFile)

	s, err := fetchSandbox(ctx, sandboxID)
	if err != nil {
		return SandboxStatus{}, err
	}
	defer s.Release()

	var contStatusList []ContainerStatus
	for _, container := range s.containers {
		contStatus, err := statusContainer(s, container.id)
		if err != nil {
			return SandboxStatus{}, err
		}

		contStatusList = append(contStatusList, contStatus)
	}

	sandboxStatus := SandboxStatus{
		ID:               s.id,
		State:            s.state,
		Hypervisor:       s.config.HypervisorType,
		HypervisorConfig: s.config.HypervisorConfig,
		Agent:            s.config.AgentType,
		ContainersStatus: contStatusList,
		Annotations:      s.config.Annotations,
	}

	return sandboxStatus, nil
}

// This function might have to stop the container if it realizes the shim
// process is not running anymore. This might be caused by two different
// reasons, either the process inside the VM exited and the shim terminated
// accordingly, or the shim has been killed directly and we need to make sure
// that we properly stop the container process inside the VM.
//
// When a container needs to be stopped because of those reasons, we want this
// to happen atomically from a sandbox perspective. That's why we cannot afford
// to take a read or read/write lock based on the situation, as it would break
// the initial locking from the caller. Instead, a read/write lock has to be
// taken from the caller, even if we simply return the container status without
// taking any action regarding the container.
func statusContainer(sandbox *Sandbox, containerID string) (ContainerStatus, error) {
	for _, container := range sandbox.containers {
		if container.id == containerID {
			// We have to check for the process state to make sure
			// we update the status in case the process is supposed
			// to be running but has been killed or terminated.
			if (container.state.State == StateReady ||
				container.state.State == StateRunning ||
				container.state.State == StatePaused) &&
				container.process.Pid > 0 {

				running, err := isShimRunning(container.process.Pid)
				if err != nil {
					return ContainerStatus{}, err
				}

				if !running {
					if err := container.stop(); err != nil {
						return ContainerStatus{}, err
					}
				}
			}

			return ContainerStatus{
				ID:          container.id,
				State:       container.state,
				PID:         container.process.Pid,
				StartTime:   container.process.StartTime,
				RootFs:      container.config.RootFs,
				Annotations: container.config.Annotations,
			}, nil
		}
	}

	// No matching containers in the sandbox
	return ContainerStatus{}, nil
}

// lock locks the sandbox to prevent it from being accessed by other processes.
func LockSandbox(ctx context.Context, sandboxID string) (*os.File, error) {
	span, ctx := trace(ctx, "StatusSandbox")
	defer span.Finish()

	fs := filesystem{}
	sandboxlockFile, _, err := fs.sandboxURI(sandboxID, lockFileType)
	if err != nil {
		return nil, err
	}

	lockFile, err := os.Open(sandboxlockFile)
	if err != nil {
		return nil, err
	}

	if err := syscall.Flock(int(lockFile.Fd()), exclusiveLock); err != nil {
		return nil, err
	}

	return lockFile, nil
}

// unlock unlocks the sandbox to allow it being accessed by other processes.
func UnlockSandbox(ctx context.Context, lockFile *os.File) error {
	span, ctx := trace(ctx, "StatusSandbox")
	defer span.Finish()

	if lockFile == nil {
		return fmt.Errorf("lockFile cannot be empty")
	}

	err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	if err != nil {
		return err
	}

	lockFile.Close()

	return nil
}
