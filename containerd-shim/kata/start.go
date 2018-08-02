// Copyright (c) 2014,2015,2016 Docker, Inc.
// Copyright (c) 2017 Intel Corporation
// Copyright (c) 2018 HyperHQ Inc.
//
// SPDX-License-Identifier: Apache-2.0
//

package kata

import (
	"fmt"
	vc "github.com/kata-containers/runtime/virtcontainers"
	"github.com/kata-containers/runtime/virtcontainers/pkg/oci"
)

func start(s *service, containerID, execID string) (vc.VCContainer, error) {
	// Checks the MUST and MUST NOT from OCI runtime specification
	status, sandboxID, err := getExistingContainerInfo(containerID)
	if err != nil {
		return nil, err
	}

	containerType, err := oci.GetContainerType(status.Annotations)
	if err != nil {
		return nil, err
	}
	if containerType.IsSandbox() {
		_, err := vc.StartSandbox(s.sandbox.ID())
		if err != nil {
			return nil, err
		}

		c := s.sandbox.GetContainer(containerID)

		if c == nil {
			return nil, fmt.Errorf("Canot get container %s from sandbox %s", containerID, containerID)
		}
		return c, nil
	}

	c, err := vci.StartContainer(sandboxID, containerID)
	if err != nil {
		return nil, err
	}

	return c, nil
}
