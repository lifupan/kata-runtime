// Copyright (c) 2018 HyperHQ Inc.
//
// SPDX-License-Identifier: Apache-2.0
//

package kata

import (
	"github.com/containerd/containerd/api/types/task"
	taskAPI "github.com/containerd/containerd/runtime/v2/task"
	vc "github.com/kata-containers/runtime/virtcontainers"
)

type Container struct {
	s        *service
	pid      uint32
	id       string
	stdin    string
	stdout   string
	stderr   string
	terminal bool
	exitch   chan struct{}

	bundle    string
	execs     map[string]*Exec
	container vc.VCContainer
	status    task.Status
	exit      uint32
}

func newContainer(s *service, r *taskAPI.CreateTaskRequest, pid uint32, container vc.VCContainer) *Container {
	c := &Container{
		s:        s,
		pid:      pid,
		id:       r.ID,
		bundle:   r.Bundle,
		stdin:    r.Stdin,
		stdout:   r.Stdout,
		stderr:   r.Stderr,
		terminal: r.Terminal,
		status:   task.StatusCreated,
		exitch:   make(chan struct{}),
	}
	return c
}
