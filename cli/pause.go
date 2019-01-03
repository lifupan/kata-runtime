// Copyright (c) 2014,2015,2016 Docker, Inc.
// Copyright (c) 2017 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package main

import (
	"context"

	"github.com/kata-containers/runtime/pkg/katautils"
	vc "github.com/kata-containers/runtime/virtcontainers"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var noteText = `Use "` + name + ` list" to identify container statuses.`

var pauseCLICommand = cli.Command{
	Name:  "pause",
	Usage: "suspend all processes in a container",
	ArgsUsage: `<container-id>

Where "<container-id>" is the container name to be paused.`,
	Description: `The pause command suspends all processes in a container.

	` + noteText,
	Action: pause,
}

var resumeCLICommand = cli.Command{
	Name:  "resume",
	Usage: "unpause all previously paused processes in a container",
	ArgsUsage: `<container-id>

Where "<container-id>" is the container name to be resumed.`,
	Description: `The resume command unpauses all processes in a container.

	` + noteText,
	Action: resume,
}

func pause(c *cli.Context) error {
	return toggle(c, true)
}

func resume(c *cli.Context) error {
	return toggle(c, false)
}

func toggle(c *cli.Context, pause bool) error {
	ctx, err := cliContextToContext(c)
	if err != nil {
		return err
	}

	return toggleContainerPause(ctx, c.Args().First(), pause)
}

func toggleContainerPause(ctx context.Context, containerID string, pause bool) (err error) {
	span, _ := katautils.Trace(ctx, "pause")
	defer span.Finish()
	span.SetTag("pause", pause)

	kataLog = kataLog.WithField("container", containerID)
	setExternalLoggers(ctx, kataLog)
	span.SetTag("container", containerID)

	// Checks the MUST and MUST NOT from OCI runtime specification
	status, sandbox, err := getExistingContainerInfo(ctx, containerID)
	if err != nil {
		return err
	}

	defer sandbox.Release()

	lockFile, err := vc.LockSandbox(ctx, sandbox.ID())
	if err != nil {
		return err
	}
	defer vc.UnlockSandbox(ctx, lockFile)

	containerID = status.ID

	kataLog = kataLog.WithFields(logrus.Fields{
		"container": containerID,
		"sandbox":   sandbox.ID(),
	})

	setExternalLoggers(ctx, kataLog)
	span.SetTag("container", containerID)
	span.SetTag("sandbox", sandbox.ID())

	if pause {
		err = sandbox.PauseContainer(containerID)
	} else {
		err = sandbox.ResumeContainer(containerID)
	}

	return err
}
