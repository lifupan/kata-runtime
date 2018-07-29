// Copyright (c) 2018 HyperHQ Inc.
//
// SPDX-License-Identifier: Apache-2.0
//

package kata

import (
	vc "github.com/kata-containers/runtime/virtcontainers"
	"github.com/containerd/containerd/api/types/task"
)

type Container struct {
	s *service
	pid     uint32
	id 		string

	bundle	string
	execs      map[string]*Exec
	container  vc.VCContainer
	status     task.Status
}

func newContainer(s *service, id, bundle string, pid uint32, container vc.VCContainer) *Container {
	c := &Container{
		s:      s,
		pid:   pid,
		id:		id,
		bundle: bundle,
	}
	return c
}
