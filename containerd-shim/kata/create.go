// Copyright (c) 2014,2015,2016 Docker, Inc.
// Copyright (c) 2017 Intel Corporation
// Copyright (c) 2018 HyperHQ Inc.
//
// SPDX-License-Identifier: Apache-2.0
//

package kata

import (
	"context"
	"fmt"
	"strings"

	vc "github.com/kata-containers/runtime/virtcontainers"
	vf "github.com/kata-containers/runtime/virtcontainers/factory"
	"github.com/kata-containers/runtime/virtcontainers/pkg/oci"

	taskAPI "github.com/containerd/containerd/runtime/v2/task"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

func create(ctx context.Context, s *service, r *taskAPI.CreateTaskRequest, pid uint32, netns string,
	runtimeConfig *oci.RuntimeConfig) (*container, error) {

	detach := !r.Terminal

	// Checks the MUST and MUST NOT from OCI runtime specification
	bundlePath, err := validCreateParams(r.ID, r.Bundle)
	if err != nil {
		return nil, err
	}

	ociSpec, err := oci.ParseConfigJSON(bundlePath)
	if err != nil {
		return nil, err
	}

	containerType, err := ociSpec.ContainerType()
	if err != nil {
		return nil, err
	}

	//In the sandbox, the containers will only
	//use the mnt space to separate the rootfs,
	//and to share the other namesapces with host
	//in the sandbox, thus remove those namespaces
	//from ocispec except networkNamespace, since
	//it has been ignored by kata-agent in sandbox.

	for _, ns := range []specs.LinuxNamespaceType{
		specs.UserNamespace,
		specs.UTSNamespace,
		specs.IPCNamespace,
		specs.PIDNamespace,
		specs.CgroupNamespace,
	} {
		removeNameSpace(&ociSpec, ns)
	}

	//set the network namespace path
	//this set will be applied to sandbox's
	//network config and hasn't nothing to
	//do with containers in the sandbox since
	//networkNamesapce hasn't been ignored by
	//kata-agent in sandbox.
	for _, n := range ociSpec.Linux.Namespaces {
		if n.Type != specs.NetworkNamespace {
			continue
		}

		if n.Path == "" {
			n.Path = netns
		}
	}

	setFactory(ctx, runtimeConfig)

	disableOutput := noNeedForOutput(detach, ociSpec.Process.Terminal)

	var c vc.VCContainer
	switch containerType {
	case vc.PodSandbox:
		if s.sandbox != nil {
			return nil, fmt.Errorf("cannot create another sandbox in sandbox: %s", s.sandbox.ID())
		}

		c, err = createSandbox(ctx, ociSpec, *runtimeConfig, r.ID, bundlePath, disableOutput)
		if err != nil {
			return nil, err
		}
		s.sandbox = c.Sandbox()

	case vc.PodContainer:
		if s.sandbox == nil {
			return nil, fmt.Errorf("BUG: Cannot start the container, since the sandbox hasn't been created")
		}

		err = createContainer(ctx, s.sandbox, ociSpec, r.ID, bundlePath, disableOutput)
		if err != nil {
			return nil, err
		}
	}

	container, err := newContainer(s, r, pid, containerType, &ociSpec)
	if err != nil {
		return nil, err
	}

	return container, nil
}

func setFactory(ctx context.Context, runtimeConfig *oci.RuntimeConfig) {
	if runtimeConfig.FactoryConfig.Template {
		factoryConfig := vf.Config{
			Template: true,
			VMConfig: vc.VMConfig{
				HypervisorType:   runtimeConfig.HypervisorType,
				HypervisorConfig: runtimeConfig.HypervisorConfig,
				AgentType:        runtimeConfig.AgentType,
				AgentConfig:      runtimeConfig.AgentConfig,
			},
		}
		logrus.WithField("factory", factoryConfig).Info("load vm factory")
		f, err := vf.NewFactory(ctx, factoryConfig, true)
		if err != nil {
			logrus.WithError(err).Warn("load vm factory failed, about to create new one")
			f, err = vf.NewFactory(ctx, factoryConfig, false)
			if err != nil {
				logrus.WithError(err).Warn("create vm factory failed")
			}
		}
		if err != nil {
			vci.SetFactory(ctx, f)
		}
	}
}

var systemdKernelParam = []vc.Param{
	{
		Key:   "systemd.unit",
		Value: systemdUnitName,
	},
	{
		Key:   "systemd.mask",
		Value: "systemd-networkd.service",
	},
	{
		Key:   "systemd.mask",
		Value: "systemd-networkd.socket",
	},
}

func getKernelParams(needSystemd bool) []vc.Param {
	p := []vc.Param{}

	if needSystemd {
		p = append(p, systemdKernelParam...)
	}

	return p
}

func needSystemd(config vc.HypervisorConfig) bool {
	return config.ImagePath != ""
}

// setKernelParams adds the user-specified kernel parameters (from the
// configuration file) to the defaults so that the former take priority.
func setKernelParams(containerID string, runtimeConfig *oci.RuntimeConfig) error {
	defaultKernelParams := getKernelParams(needSystemd(runtimeConfig.HypervisorConfig))

	if runtimeConfig.HypervisorConfig.Debug {
		strParams := vc.SerializeParams(defaultKernelParams, "=")
		formatted := strings.Join(strParams, " ")

		logrus.WithField("default-kernel-parameters", formatted).Debug()
	}

	// retrieve the parameters specified in the config file
	userKernelParams := runtimeConfig.HypervisorConfig.KernelParams

	// reset
	runtimeConfig.HypervisorConfig.KernelParams = []vc.Param{}

	// first, add default values
	for _, p := range defaultKernelParams {
		if err := (runtimeConfig).AddKernelParam(p); err != nil {
			return err
		}
	}

	// now re-add the user-specified values so that they take priority.
	for _, p := range userKernelParams {
		if err := (runtimeConfig).AddKernelParam(p); err != nil {
			return err
		}
	}

	return nil
}

func createSandbox(ctx context.Context, ociSpec oci.CompatOCISpec, runtimeConfig oci.RuntimeConfig,
	containerID, bundlePath string, disableOutput bool) (vc.VCContainer, error) {

	err := setKernelParams(containerID, &runtimeConfig)
	if err != nil {
		return nil, err
	}

	sandboxConfig, err := oci.SandboxConfig(ociSpec, runtimeConfig, bundlePath, containerID, "", disableOutput, false)
	if err != nil {
		return nil, err
	}

	sandboxConfig.Stateful = true

	//setup the networkNamespace if it hasn't been created, such as the using the CNM
	if err = setupNetworkNamespace(&sandboxConfig.NetworkConfig); err != nil {
		return nil, err
	}

	// Run pre-start OCI hooks.
	err = enterNetNS(sandboxConfig.NetworkConfig.NetNSPath, func() error {
		return preStartHooks(ctx, ociSpec, containerID, bundlePath)
	})
	if err != nil {
		return nil, err
	}

	sandbox, err := vci.CreateSandbox(ctx, sandboxConfig)
	if err != nil {
		return nil, err
	}

	containers := sandbox.GetAllContainers()
	if len(containers) != 1 {
		return nil, fmt.Errorf("BUG: Container list from sandbox is wrong, expecting only one container, found %d containers", len(containers))
	}

	return containers[0], nil
}

// setEphemeralStorageType sets the mount type to 'ephemeral'
// if the mount source path is provisioned by k8s for ephemeral storage.
// For the given pod ephemeral volume is created only once
// backed by tmpfs inside the VM. For successive containers
// of the same pod the already existing volume is reused.
func setEphemeralStorageType(ociSpec oci.CompatOCISpec) oci.CompatOCISpec {
	for idx, mnt := range ociSpec.Mounts {
		if IsEphemeralStorage(mnt.Source) {
			ociSpec.Mounts[idx].Type = "ephemeral"
		}
	}
	return ociSpec
}

func createContainer(ctx context.Context, sandbox vc.VCSandbox, ociSpec oci.CompatOCISpec, containerID, bundlePath string,
	disableOutput bool) error {

	ociSpec = setEphemeralStorageType(ociSpec)

	contConfig, err := oci.ContainerConfig(ociSpec, bundlePath, containerID, "", disableOutput)
	if err != nil {
		return err
	}

	// Run pre-start OCI hooks.
	err = enterNetNS(sandbox.GetNetNs(), func() error {
		return preStartHooks(ctx, ociSpec, containerID, bundlePath)
	})
	if err != nil {
		return err
	}

	_, err = sandbox.CreateContainer(contConfig)
	if err != nil {
		return err
	}

	return nil
}
