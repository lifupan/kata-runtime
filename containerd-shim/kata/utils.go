// Copyright (c) 2017 Intel Corporation
// Copyright (c) 2018 HyperHQ Inc.
//
// SPDX-License-Identifier: Apache-2.0
//

package kata

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd/mount"
	cdshim "github.com/containerd/containerd/runtime/v2/shim"

	vc "github.com/kata-containers/runtime/virtcontainers"
	"github.com/kata-containers/runtime/virtcontainers/pkg/oci"

	"github.com/sirupsen/logrus"
)

const (
	k8sEmptyDir = "kubernetes.io~empty-dir"
)

// VSockDevicePath path to vsock device
var VSockDevicePath = "/dev/vsock"

// VHostVSockDevicePath path to vhost-vsock device
var VHostVSockDevicePath = "/dev/vhost-vsock"

// IsEphemeralStorage returns true if the given path
// to the storage belongs to kubernetes ephemeral storage
//
// This method depends on a specific path used by k8s
// to detect if it's of type ephemeral. As of now,
// this is a very k8s specific solution that works
// but in future there should be a better way for this
// method to determine if the path is for ephemeral
// volume type
func IsEphemeralStorage(path string) bool {
	splitSourceSlice := strings.Split(path, "/")
	if len(splitSourceSlice) > 1 {
		storageType := splitSourceSlice[len(splitSourceSlice)-2]
		if storageType == k8sEmptyDir {
			return true
		}
	}
	return false
}

// resolvePath returns the fully resolved and expanded value of the
// specified path.
func resolvePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path must be specified")
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		if os.IsNotExist(err) {
			// Make the error clearer than the default
			return "", fmt.Errorf("file %v does not exist", absolute)
		}

		return "", err
	}

	return resolved, nil
}

func cReap(s *service, pid, status int, id, execid string, exitat time.Time) {
	s.ec <- exit{
		timestamp: exitat,
		pid:       pid,
		status:    status,
		id:        id,
		execid:    execid,
	}
}

func getAddress(ctx context.Context, bundlePath, id string) (string, error) {
	var err error

	// Checks the MUST and MUST NOT from OCI runtime specification
	if bundlePath, err = validCreateParams(id, bundlePath); err != nil {
		return "", err
	}

	ociSpec, err := oci.ParseConfigJSON(bundlePath)
	if err != nil {
		return "", err
	}

	containerType, err := ociSpec.ContainerType()
	if err != nil {
		return "", err
	}

	if containerType == vc.PodContainer {
		sandboxID, err := ociSpec.SandboxID()
		if err != nil {
			return "", err
		}
		address, err := cdshim.SocketAddress(ctx, sandboxID)
		if err != nil {
			return "", err
		}
		return address, nil
	}

	return "", nil
}

func cleanupContainer(ctx context.Context, sid, cid, bundlePath string) error {
	logrus.WithField("Service", "Cleanup").WithField("container", cid).Info("Cleanup container")

	rootfs := filepath.Join(bundlePath, "rootfs")
	sandbox, err := vci.FetchSandbox(ctx, sid)
	if err != nil {
		return err
	}

	status, err := sandbox.StatusContainer(cid)
	if err != nil {
		logrus.WithError(err).WithField("container", cid).Warn("failed to get container status")
		return err
	}

	if oci.StateToOCIState(status.State) != oci.StateStopped {
		err := vci.KillContainer(ctx, sid, cid, syscall.SIGKILL, true)
		if err != nil {
			logrus.WithError(err).WithField("container", cid).Warn("failed to kill container")
			return err
		}
	}

	if _, err = vci.StopContainer(ctx, sid, cid); err != nil {
		logrus.WithError(err).WithField("container", cid).Warn("failed to stop container")
		return err
	}

	if _, err := vci.DeleteContainer(ctx, sid, cid); err != nil {
		logrus.WithError(err).WithField("container", cid).Warn("failed to remove container")
	}

	if err := mount.UnmountAll(rootfs, 0); err != nil {
		logrus.WithError(err).WithField("container", cid).Warn("failed to cleanup container rootfs")
	}

	if len(sandbox.GetAllContainers()) == 0 {
		_, err = vci.StopSandbox(ctx, sid)
		if err != nil {
			logrus.WithError(err).WithField("sandbox", sid).Warn("failed to stop sandbox")
			return err
		}

		_, err = vci.DeleteSandbox(ctx, sid)
		if err != nil {
			logrus.WithError(err).WithField("sandbox", sid).Warnf("failed to delete sandbox")
			return err
		}
	}

	return nil
}

// SupportsVsocks returns true if vsocks are supported, otherwise false
func SupportsVsocks() bool {
	if _, err := os.Stat(VSockDevicePath); err != nil {
		return false
	}

	if _, err := os.Stat(VHostVSockDevicePath); err != nil {
		return false
	}

	return true
}
