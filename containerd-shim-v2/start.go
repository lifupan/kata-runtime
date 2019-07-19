// Copyright (c) 2018 HyperHQ Inc.
//
// SPDX-License-Identifier: Apache-2.0
//

package containerdshim

import (
	"context"
	"fmt"
	"github.com/containerd/containerd/api/types/task"
	"github.com/kata-containers/runtime/pkg/katautils"
)

func (c *container) startContainer(ctx context.Context, s *service) error {
	//start a container
	if c.cType == "" {
		err := fmt.Errorf("Bug, the container %s type is empty", c.id)
		return err
	}

	if s.sandbox == nil {
		err := fmt.Errorf("Bug, the sandbox hasn't been created for this container %s", c.id)
		return err
	}

	if c.cType.IsSandbox() {
		err := s.sandbox.Start()
		if err != nil {
			return err
		}
	} else {
		_, err := s.sandbox.StartContainer(c.id)
		if err != nil {
			return err
		}
	}

	// Run post-start OCI hooks.
	err := katautils.EnterNetNS(s.sandbox.GetNetNs(), func() error {
		return katautils.PostStartHooks(ctx, *c.spec, s.sandbox.ID(), c.bundle)
	})
	if err != nil {
		return err
	}

	c.status = task.StatusRunning
	err = c.ioCopyWait(ctx, s)

	return err
}

func (execs *exec) startExec(ctx context.Context, s *service, containerID, execID string) error {
	_, proc, err := s.sandbox.EnterContainer(containerID, *execs.cmds)
	if err != nil {
		err := fmt.Errorf("cannot enter container %s, with err %s", containerID, err)
		return err
	}
	execs.id = proc.Token

	execs.status = task.StatusRunning
	if execs.tty.Height != 0 && execs.tty.Width != 0 {
		err = s.sandbox.WinsizeProcess(containerID, execs.id, execs.tty.Height, execs.tty.Width)
		if err != nil {
			return err
		}
	}

	err = execs.ioCopyWait(ctx, s, execID)

	return err
}
