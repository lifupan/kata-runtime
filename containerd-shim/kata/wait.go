// Copyright (c) 2018 HyperHQ Inc.
//
// SPDX-License-Identifier: Apache-2.0
//

package kata

import (
	"time"
	"github.com/containerd/containerd/api/types/task"
)

func wait(s *service, c *Container, execID string) (int32, error){

	processID := execID
	if processID == "" {
		processID = c.id
		//wait until the io closed, then wait the container
		<-c.exitch
	}

	ret, err := s.sandbox.WaitProcess(c.id, processID)
	if err != nil {
		return ret, err
	}

	c.mu.Lock()
	c.status = task.StatusStopped
	c.exit = uint32(ret)
	c.time = time.Now()
	c.mu.Unlock()

	go cReap(s, int(c.pid), int(ret), c.id, execID, c.time)

	return ret, nil
}
