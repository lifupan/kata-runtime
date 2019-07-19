// Copyright (c) 2018 HyperHQ Inc.
//
// SPDX-License-Identifier: Apache-2.0
//

package containerdshim

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/typeurl"
	googleProtobuf "github.com/gogo/protobuf/types"
	"github.com/kata-containers/runtime/virtcontainers/types"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type exec struct {
	container *container
	cmds      *types.Cmd
	tty       *tty
	ttyio     *ttyIO
	id        string

	exitCode int32

	status task.Status

	exitIOch chan struct{}
	exitCh   chan uint32

	exitTime time.Time
}

type execState struct {
	Id     string      `json:"id"`
	Tty    tty         `json:"tty"`
	Status task.Status `json:"status"`
}

type tty struct {
	Stdin    string `json:"stdin"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Height   uint32 `json:"height"`
	Width    uint32 `json:"width"`
	Terminal bool   `json:"terminal"`
}

func getEnvs(envs []string) []types.EnvVar {
	var vcEnvs = []types.EnvVar{}
	var env types.EnvVar

	for _, v := range envs {
		pair := strings.SplitN(v, "=", 2)

		if len(pair) == 2 {
			env = types.EnvVar{Var: pair[0], Value: pair[1]}
		} else if len(pair) == 1 {
			env = types.EnvVar{Var: pair[0], Value: ""}
		}

		vcEnvs = append(vcEnvs, env)
	}

	return vcEnvs
}

func newExec(c *container, stdin, stdout, stderr string, terminal, init bool, jspec *googleProtobuf.Any) (*exec, error) {
	var height uint32
	var width uint32
	var cmds *types.Cmd

	if init && jspec == nil {
		return nil, errdefs.ToGRPCf(errdefs.ErrInvalidArgument, "googleProtobuf.Any points to nil")
	}

	if jspec != nil {
		// process exec request
		var spec *specs.Process
		v, err := typeurl.UnmarshalAny(jspec)
		if err != nil {
			return nil, err
		}
		spec, ok := v.(*specs.Process)
		if !ok {
			return nil, errdefs.ToGRPCf(errdefs.ErrInvalidArgument, "Get an invalid spec type")
		}

		if spec.ConsoleSize != nil {
			height = uint32(spec.ConsoleSize.Height)
			width = uint32(spec.ConsoleSize.Width)
		}

		cmds = &types.Cmd{
			Args:            spec.Args,
			Envs:            getEnvs(spec.Env),
			User:            fmt.Sprintf("%d", spec.User.UID),
			PrimaryGroup:    fmt.Sprintf("%d", spec.User.GID),
			WorkDir:         spec.Cwd,
			Interactive:     terminal,
			Detach:          !terminal,
			NoNewPrivileges: spec.NoNewPrivileges,
		}
	}

	tty := &tty{
		Stdin:    stdin,
		Stdout:   stdout,
		Stderr:   stderr,
		Height:   height,
		Width:    width,
		Terminal: terminal,
	}

	exec := &exec{
		container: c,
		cmds:      cmds,
		tty:       tty,
		exitCode:  exitCode255,
		exitIOch:  make(chan struct{}),
		exitCh:    make(chan uint32, 1),
		status:    task.StatusCreated,
	}

	return exec, nil
}

func (c *container) getExec(id string) (*exec, error) {
	if c.execs == nil {
		return nil, errdefs.ToGRPCf(errdefs.ErrNotFound, "exec does not exist %s", id)
	}

	exec := c.execs[id]

	if exec == nil {
		return nil, errdefs.ToGRPCf(errdefs.ErrNotFound, "exec does not exist %s", id)
	}

	return exec, nil
}

func (e *exec) ioCopyWait(ctx context.Context, s *service, execID string) error {
	c := e.container

	stdin, stdout, stderr, err := s.sandbox.IOStream(c.id, e.id)
	if err != nil {
		return err
	}
	tty, err := newTtyIO(ctx, e.tty.Stdin, e.tty.Stdout, e.tty.Stderr, e.tty.Terminal)
	if err != nil {
		return err
	}
	e.ttyio = tty

	go ioCopy(e.exitIOch, tty, stdin, stdout, stderr)

	go wait(s, c, execID)

	return nil
}
