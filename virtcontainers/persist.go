// Copyright (c) 2019 Huawei Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package virtcontainers

import (
	"context"
	"errors"

	"github.com/kata-containers/runtime/virtcontainers/device/api"
	exp "github.com/kata-containers/runtime/virtcontainers/experimental"
	"github.com/kata-containers/runtime/virtcontainers/persist"
	persistapi "github.com/kata-containers/runtime/virtcontainers/persist/api"
	"github.com/kata-containers/runtime/virtcontainers/store"
	"github.com/kata-containers/runtime/virtcontainers/types"
	"github.com/mitchellh/mapstructure"
)

var (
	errContainerPersistNotExist = errors.New("container doesn't exist in persist data")
)

func (s *Sandbox) dumpVersion(ss *persistapi.SandboxState) {
	// New created sandbox has a uninitialized `PersistVersion` which should be set to current version when do the first saving;
	// Old restored sandbox should keep its original version and shouldn't be modified any more after it's initialized.
	ss.PersistVersion = s.state.PersistVersion
	if ss.PersistVersion == 0 {
		ss.PersistVersion = persistapi.CurPersistVersion
	}
}

func (s *Sandbox) dumpState(ss *persistapi.SandboxState, cs map[string]persistapi.ContainerState) {
	ss.SandboxContainer = s.id
	ss.GuestMemoryBlockSizeMB = s.state.GuestMemoryBlockSizeMB
	ss.GuestMemoryHotplugProbe = s.state.GuestMemoryHotplugProbe
	ss.State = string(s.state.State)
	ss.CgroupPath = s.state.CgroupPath
	ss.CgroupPaths = s.state.CgroupPaths

	for id, cont := range s.containers {
		state := persistapi.ContainerState{}
		if v, ok := cs[id]; ok {
			state = v
		}
		state.State = string(cont.state.State)
		state.Rootfs = persistapi.RootfsState{
			BlockDeviceID: cont.state.BlockDeviceID,
			FsType:        cont.state.Fstype,
		}
		state.CgroupPath = cont.state.CgroupPath
		cs[id] = state
	}

	// delete removed containers
	for id := range cs {
		if _, ok := s.containers[id]; !ok {
			delete(cs, id)
		}
	}
}

func (s *Sandbox) dumpHypervisor(ss *persistapi.SandboxState) {
	ss.HypervisorState = s.hypervisor.save()
	// BlockIndexMap will be moved from sandbox state to hypervisor state later
	ss.HypervisorState.BlockIndexMap = s.state.BlockIndexMap
}

func deviceToDeviceState(devices []api.Device) (dss []persistapi.DeviceState) {
	for _, dev := range devices {
		dss = append(dss, dev.Save())
	}
	return
}

func (s *Sandbox) dumpDevices(ss *persistapi.SandboxState, cs map[string]persistapi.ContainerState) {
	ss.Devices = deviceToDeviceState(s.devManager.GetAllDevices())

	for id, cont := range s.containers {
		state := persistapi.ContainerState{}
		if v, ok := cs[id]; ok {
			state = v
		}

		state.DeviceMaps = nil
		for _, dev := range cont.devices {
			state.DeviceMaps = append(state.DeviceMaps, persistapi.DeviceMap{
				ID:            dev.ID,
				ContainerPath: dev.ContainerPath,
				FileMode:      dev.FileMode,
				UID:           dev.UID,
				GID:           dev.GID,
			})
		}

		cs[id] = state
	}

	// delete removed containers
	for id := range cs {
		if _, ok := s.containers[id]; !ok {
			delete(cs, id)
		}
	}
}

func (s *Sandbox) dumpProcess(cs map[string]persistapi.ContainerState) {
	for id, cont := range s.containers {
		state := persistapi.ContainerState{}
		if v, ok := cs[id]; ok {
			state = v
		}

		state.Process = persistapi.Process{
			Token:     cont.process.Token,
			Pid:       cont.process.Pid,
			StartTime: cont.process.StartTime,
		}

		cs[id] = state
	}

	// delete removed containers
	for id := range cs {
		if _, ok := s.containers[id]; !ok {
			delete(cs, id)
		}
	}
}

func (s *Sandbox) dumpMounts(cs map[string]persistapi.ContainerState) {
	for id, cont := range s.containers {
		state := persistapi.ContainerState{}
		if v, ok := cs[id]; ok {
			state = v
		}

		for _, m := range cont.mounts {
			state.Mounts = append(state.Mounts, persistapi.Mount{
				Source:        m.Source,
				Destination:   m.Destination,
				Options:       m.Options,
				HostPath:      m.HostPath,
				ReadOnly:      m.ReadOnly,
				BlockDeviceID: m.BlockDeviceID,
			})
		}

		cs[id] = state
	}

	// delete removed containers
	for id := range cs {
		if _, ok := s.containers[id]; !ok {
			delete(cs, id)
		}
	}
}

func (s *Sandbox) dumpAgent(ss *persistapi.SandboxState) {
	if s.agent != nil {
		ss.AgentState = s.agent.save()
	}
}

func (s *Sandbox) dumpNetwork(ss *persistapi.SandboxState) {
	ss.Network = persistapi.NetworkInfo{
		NetNsPath:    s.networkNS.NetNsPath,
		NetmonPID:    s.networkNS.NetmonPID,
		NetNsCreated: s.networkNS.NetNsCreated,
	}
	for _, e := range s.networkNS.Endpoints {
		ss.Network.Endpoints = append(ss.Network.Endpoints, e.save())
	}
}

func (s *Sandbox) dumpConfig(ss *persistapi.SandboxState) {
	sconfig := s.config
	ss.Config = persistapi.SandboxConfig{
		HypervisorType: string(sconfig.HypervisorType),
		AgentType:      string(sconfig.AgentType),
		ProxyType:      string(sconfig.ProxyType),
		ProxyConfig: persistapi.ProxyConfig{
			Path:  sconfig.ProxyConfig.Path,
			Debug: sconfig.ProxyConfig.Debug,
		},
		ShimType: string(sconfig.ShimType),
		NetworkConfig: persistapi.NetworkConfig{
			NetNSPath:         sconfig.NetworkConfig.NetNSPath,
			NetNsCreated:      sconfig.NetworkConfig.NetNsCreated,
			DisableNewNetNs:   sconfig.NetworkConfig.DisableNewNetNs,
			InterworkingModel: int(sconfig.NetworkConfig.InterworkingModel),
		},

		ShmSize:             sconfig.ShmSize,
		SharePidNs:          sconfig.SharePidNs,
		Stateful:            sconfig.Stateful,
		SystemdCgroup:       sconfig.SystemdCgroup,
		SandboxCgroupOnly:   sconfig.SandboxCgroupOnly,
		EnableAgentPidNs:    sconfig.EnableAgentPidNs,
		DisableGuestSeccomp: sconfig.DisableGuestSeccomp,
		Cgroups:             sconfig.Cgroups,
	}

	for _, e := range sconfig.Experimental {
		ss.Config.Experimental = append(ss.Config.Experimental, e.Name)
	}

	ss.Config.HypervisorConfig = persistapi.HypervisorConfig{
		NumVCPUs:                sconfig.HypervisorConfig.NumVCPUs,
		DefaultMaxVCPUs:         sconfig.HypervisorConfig.DefaultMaxVCPUs,
		MemorySize:              sconfig.HypervisorConfig.MemorySize,
		DefaultBridges:          sconfig.HypervisorConfig.DefaultBridges,
		Msize9p:                 sconfig.HypervisorConfig.Msize9p,
		MemSlots:                sconfig.HypervisorConfig.MemSlots,
		MemOffset:               sconfig.HypervisorConfig.MemOffset,
		VirtioMem:               sconfig.HypervisorConfig.VirtioMem,
		VirtioFSCacheSize:       sconfig.HypervisorConfig.VirtioFSCacheSize,
		KernelPath:              sconfig.HypervisorConfig.KernelPath,
		ImagePath:               sconfig.HypervisorConfig.ImagePath,
		InitrdPath:              sconfig.HypervisorConfig.InitrdPath,
		FirmwarePath:            sconfig.HypervisorConfig.FirmwarePath,
		MachineAccelerators:     sconfig.HypervisorConfig.MachineAccelerators,
		CPUFeatures:             sconfig.HypervisorConfig.CPUFeatures,
		HypervisorPath:          sconfig.HypervisorConfig.HypervisorPath,
		HypervisorCtlPath:       sconfig.HypervisorConfig.HypervisorCtlPath,
		JailerPath:              sconfig.HypervisorConfig.JailerPath,
		BlockDeviceDriver:       sconfig.HypervisorConfig.BlockDeviceDriver,
		HypervisorMachineType:   sconfig.HypervisorConfig.HypervisorMachineType,
		MemoryPath:              sconfig.HypervisorConfig.MemoryPath,
		DevicesStatePath:        sconfig.HypervisorConfig.DevicesStatePath,
		EntropySource:           sconfig.HypervisorConfig.EntropySource,
		SharedFS:                sconfig.HypervisorConfig.SharedFS,
		VirtioFSDaemon:          sconfig.HypervisorConfig.VirtioFSDaemon,
		VirtioFSCache:           sconfig.HypervisorConfig.VirtioFSCache,
		VirtioFSExtraArgs:       sconfig.HypervisorConfig.VirtioFSExtraArgs[:],
		BlockDeviceCacheSet:     sconfig.HypervisorConfig.BlockDeviceCacheSet,
		BlockDeviceCacheDirect:  sconfig.HypervisorConfig.BlockDeviceCacheDirect,
		BlockDeviceCacheNoflush: sconfig.HypervisorConfig.BlockDeviceCacheNoflush,
		DisableBlockDeviceUse:   sconfig.HypervisorConfig.DisableBlockDeviceUse,
		EnableIOThreads:         sconfig.HypervisorConfig.EnableIOThreads,
		Debug:                   sconfig.HypervisorConfig.Debug,
		MemPrealloc:             sconfig.HypervisorConfig.MemPrealloc,
		HugePages:               sconfig.HypervisorConfig.HugePages,
		FileBackedMemRootDir:    sconfig.HypervisorConfig.FileBackedMemRootDir,
		Realtime:                sconfig.HypervisorConfig.Realtime,
		Mlock:                   sconfig.HypervisorConfig.Mlock,
		DisableNestingChecks:    sconfig.HypervisorConfig.DisableNestingChecks,
		UseVSock:                sconfig.HypervisorConfig.UseVSock,
		DisableImageNvdimm:      sconfig.HypervisorConfig.DisableImageNvdimm,
		HotplugVFIOOnRootBus:    sconfig.HypervisorConfig.HotplugVFIOOnRootBus,
		PCIeRootPort:            sconfig.HypervisorConfig.PCIeRootPort,
		BootToBeTemplate:        sconfig.HypervisorConfig.BootToBeTemplate,
		BootFromTemplate:        sconfig.HypervisorConfig.BootFromTemplate,
		DisableVhostNet:         sconfig.HypervisorConfig.DisableVhostNet,
		EnableVhostUserStore:    sconfig.HypervisorConfig.EnableVhostUserStore,
		VhostUserStorePath:      sconfig.HypervisorConfig.VhostUserStorePath,
		GuestHookPath:           sconfig.HypervisorConfig.GuestHookPath,
		VMid:                    sconfig.HypervisorConfig.VMid,
	}

	if sconfig.AgentType == "kata" {
		var sagent KataAgentConfig
		err := mapstructure.Decode(sconfig.AgentConfig, &sagent)
		if err != nil {
			s.Logger().WithError(err).Error("internal error: KataAgentConfig failed to decode")
		} else {
			ss.Config.KataAgentConfig = &persistapi.KataAgentConfig{
				LongLiveConn: sagent.LongLiveConn,
				UseVSock:     sagent.UseVSock,
			}
		}
	}

	if sconfig.ShimType == "kataShim" {
		var shim ShimConfig
		err := mapstructure.Decode(sconfig.ShimConfig, &shim)
		if err != nil {
			s.Logger().WithError(err).Error("internal error: ShimConfig failed to decode")
		} else {
			ss.Config.KataShimConfig = &persistapi.ShimConfig{
				Path:  shim.Path,
				Debug: shim.Debug,
			}
		}
	}

	for _, contConf := range sconfig.Containers {
		ss.Config.ContainerConfigs = append(ss.Config.ContainerConfigs, persistapi.ContainerConfig{
			ID:          contConf.ID,
			Annotations: contConf.Annotations,
			RootFs:      contConf.RootFs.Target,
			Resources:   contConf.Resources,
		})
	}
}

func (s *Sandbox) Save() error {
	var (
		ss = persistapi.SandboxState{}
		cs = make(map[string]persistapi.ContainerState)
	)

	s.dumpVersion(&ss)
	s.dumpState(&ss, cs)
	s.dumpHypervisor(&ss)
	s.dumpDevices(&ss, cs)
	s.dumpProcess(cs)
	s.dumpMounts(cs)
	s.dumpAgent(&ss)
	s.dumpNetwork(&ss)
	s.dumpConfig(&ss)

	if err := s.newStore.ToDisk(ss, cs); err != nil {
		return err
	}

	return nil
}

func (s *Sandbox) loadState(ss persistapi.SandboxState) {
	s.state.PersistVersion = ss.PersistVersion
	s.state.GuestMemoryBlockSizeMB = ss.GuestMemoryBlockSizeMB
	s.state.BlockIndexMap = ss.HypervisorState.BlockIndexMap
	s.state.State = types.StateString(ss.State)
	s.state.CgroupPath = ss.CgroupPath
	s.state.CgroupPaths = ss.CgroupPaths
	s.state.GuestMemoryHotplugProbe = ss.GuestMemoryHotplugProbe
}

func (c *Container) loadContState(cs persistapi.ContainerState) {
	c.state = types.ContainerState{
		State:         types.StateString(cs.State),
		BlockDeviceID: cs.Rootfs.BlockDeviceID,
		Fstype:        cs.Rootfs.FsType,
		CgroupPath:    cs.CgroupPath,
	}
}

func (s *Sandbox) loadHypervisor(hs persistapi.HypervisorState) {
	s.hypervisor.load(hs)
}

func (s *Sandbox) loadAgent(as persistapi.AgentState) {
	if s.agent != nil {
		s.agent.load(as)
	}
}

func (s *Sandbox) loadDevices(devStates []persistapi.DeviceState) {
	s.devManager.LoadDevices(devStates)
}

func (c *Container) loadContDevices(cs persistapi.ContainerState) {
	c.devices = nil
	for _, dev := range cs.DeviceMaps {
		c.devices = append(c.devices, ContainerDevice{
			ID:            dev.ID,
			ContainerPath: dev.ContainerPath,
			FileMode:      dev.FileMode,
			UID:           dev.UID,
			GID:           dev.GID,
		})
	}
}

func (c *Container) loadContMounts(cs persistapi.ContainerState) {
	c.mounts = nil
	for _, m := range cs.Mounts {
		c.mounts = append(c.mounts, Mount{
			Source:        m.Source,
			Destination:   m.Destination,
			Options:       m.Options,
			HostPath:      m.HostPath,
			ReadOnly:      m.ReadOnly,
			BlockDeviceID: m.BlockDeviceID,
		})
	}
}

func (c *Container) loadContProcess(cs persistapi.ContainerState) {
	c.process = Process{
		Token:     cs.Process.Token,
		Pid:       cs.Process.Pid,
		StartTime: cs.Process.StartTime,
	}
}

func (s *Sandbox) loadNetwork(netInfo persistapi.NetworkInfo) {
	s.networkNS = NetworkNamespace{
		NetNsPath:    netInfo.NetNsPath,
		NetmonPID:    netInfo.NetmonPID,
		NetNsCreated: netInfo.NetNsCreated,
	}

	for _, e := range netInfo.Endpoints {
		var ep Endpoint
		switch EndpointType(e.Type) {
		case PhysicalEndpointType:
			ep = &PhysicalEndpoint{}
		case VethEndpointType:
			ep = &VethEndpoint{}
		case VhostUserEndpointType:
			ep = &VhostUserEndpoint{}
		case BridgedMacvlanEndpointType:
			ep = &BridgedMacvlanEndpoint{}
		case MacvtapEndpointType:
			ep = &MacvtapEndpoint{}
		case TapEndpointType:
			ep = &TapEndpoint{}
		case IPVlanEndpointType:
			ep = &IPVlanEndpoint{}
		default:
			s.Logger().WithField("endpoint-type", e.Type).Error("unknown endpoint type")
			continue
		}
		ep.load(e)
		s.networkNS.Endpoints = append(s.networkNS.Endpoints, ep)
	}
}

// Restore will restore sandbox data from persist file on disk
func (s *Sandbox) Restore() error {
	ss, _, err := s.newStore.FromDisk(s.id)
	if err != nil {
		return err
	}

	s.loadState(ss)
	s.loadHypervisor(ss.HypervisorState)
	s.loadDevices(ss.Devices)
	s.loadAgent(ss.AgentState)
	s.loadNetwork(ss.Network)
	return nil
}

// Restore will restore container data from persist file on disk
func (c *Container) Restore() error {
	_, css, err := c.sandbox.newStore.FromDisk(c.sandbox.id)
	if err != nil {
		return err
	}

	cs, ok := css[c.id]
	if !ok {
		return errContainerPersistNotExist
	}

	c.loadContState(cs)
	c.loadContDevices(cs)
	c.loadContProcess(cs)
	c.loadContMounts(cs)
	return nil
}

func loadSandboxConfig(id string) (*SandboxConfig, error) {
	store, err := persist.GetDriver()
	if err != nil || store == nil {
		return nil, errors.New("failed to get fs persist driver")
	}

	ss, _, err := store.FromDisk(id)
	if err != nil {
		return nil, err
	}

	savedConf := ss.Config
	sconfig := &SandboxConfig{
		ID:             id,
		HypervisorType: HypervisorType(savedConf.HypervisorType),
		AgentType:      AgentType(savedConf.AgentType),
		ProxyType:      ProxyType(savedConf.ProxyType),
		ProxyConfig: ProxyConfig{
			Path:  savedConf.ProxyConfig.Path,
			Debug: savedConf.ProxyConfig.Debug,
		},
		ShimType: ShimType(savedConf.ShimType),
		NetworkConfig: NetworkConfig{
			NetNSPath:         savedConf.NetworkConfig.NetNSPath,
			NetNsCreated:      savedConf.NetworkConfig.NetNsCreated,
			DisableNewNetNs:   savedConf.NetworkConfig.DisableNewNetNs,
			InterworkingModel: NetInterworkingModel(savedConf.NetworkConfig.InterworkingModel),
		},

		ShmSize:             savedConf.ShmSize,
		SharePidNs:          savedConf.SharePidNs,
		Stateful:            savedConf.Stateful,
		SystemdCgroup:       savedConf.SystemdCgroup,
		SandboxCgroupOnly:   savedConf.SandboxCgroupOnly,
		EnableAgentPidNs:    savedConf.EnableAgentPidNs,
		DisableGuestSeccomp: savedConf.DisableGuestSeccomp,
		Cgroups:             savedConf.Cgroups,
	}

	for _, name := range savedConf.Experimental {
		sconfig.Experimental = append(sconfig.Experimental, *exp.Get(name))
	}

	hconf := savedConf.HypervisorConfig
	sconfig.HypervisorConfig = HypervisorConfig{
		NumVCPUs:                hconf.NumVCPUs,
		DefaultMaxVCPUs:         hconf.DefaultMaxVCPUs,
		MemorySize:              hconf.MemorySize,
		DefaultBridges:          hconf.DefaultBridges,
		Msize9p:                 hconf.Msize9p,
		MemSlots:                hconf.MemSlots,
		MemOffset:               hconf.MemOffset,
		VirtioMem:               hconf.VirtioMem,
		VirtioFSCacheSize:       hconf.VirtioFSCacheSize,
		KernelPath:              hconf.KernelPath,
		ImagePath:               hconf.ImagePath,
		InitrdPath:              hconf.InitrdPath,
		FirmwarePath:            hconf.FirmwarePath,
		MachineAccelerators:     hconf.MachineAccelerators,
		CPUFeatures:             hconf.CPUFeatures,
		HypervisorPath:          hconf.HypervisorPath,
		HypervisorCtlPath:       hconf.HypervisorCtlPath,
		JailerPath:              hconf.JailerPath,
		BlockDeviceDriver:       hconf.BlockDeviceDriver,
		HypervisorMachineType:   hconf.HypervisorMachineType,
		MemoryPath:              hconf.MemoryPath,
		DevicesStatePath:        hconf.DevicesStatePath,
		EntropySource:           hconf.EntropySource,
		SharedFS:                hconf.SharedFS,
		VirtioFSDaemon:          hconf.VirtioFSDaemon,
		VirtioFSCache:           hconf.VirtioFSCache,
		VirtioFSExtraArgs:       hconf.VirtioFSExtraArgs[:],
		BlockDeviceCacheSet:     hconf.BlockDeviceCacheSet,
		BlockDeviceCacheDirect:  hconf.BlockDeviceCacheDirect,
		BlockDeviceCacheNoflush: hconf.BlockDeviceCacheNoflush,
		DisableBlockDeviceUse:   hconf.DisableBlockDeviceUse,
		EnableIOThreads:         hconf.EnableIOThreads,
		Debug:                   hconf.Debug,
		MemPrealloc:             hconf.MemPrealloc,
		HugePages:               hconf.HugePages,
		FileBackedMemRootDir:    hconf.FileBackedMemRootDir,
		Realtime:                hconf.Realtime,
		Mlock:                   hconf.Mlock,
		DisableNestingChecks:    hconf.DisableNestingChecks,
		UseVSock:                hconf.UseVSock,
		DisableImageNvdimm:      hconf.DisableImageNvdimm,
		HotplugVFIOOnRootBus:    hconf.HotplugVFIOOnRootBus,
		PCIeRootPort:            hconf.PCIeRootPort,
		BootToBeTemplate:        hconf.BootToBeTemplate,
		BootFromTemplate:        hconf.BootFromTemplate,
		DisableVhostNet:         hconf.DisableVhostNet,
		EnableVhostUserStore:    hconf.EnableVhostUserStore,
		VhostUserStorePath:      hconf.VhostUserStorePath,
		GuestHookPath:           hconf.GuestHookPath,
		VMid:                    hconf.VMid,
	}

	if savedConf.AgentType == "kata" {
		sconfig.AgentConfig = KataAgentConfig{
			LongLiveConn: savedConf.KataAgentConfig.LongLiveConn,
			UseVSock:     savedConf.KataAgentConfig.UseVSock,
		}
	}

	if savedConf.ShimType == "kataShim" {
		sconfig.ShimConfig = ShimConfig{
			Path:  savedConf.KataShimConfig.Path,
			Debug: savedConf.KataShimConfig.Debug,
		}
	}

	for _, contConf := range savedConf.ContainerConfigs {
		sconfig.Containers = append(sconfig.Containers, ContainerConfig{
			ID:          contConf.ID,
			Annotations: contConf.Annotations,
			Resources:   contConf.Resources,
			RootFs: RootFs{
				Target: contConf.RootFs,
			},
		})
	}
	return sconfig, nil
}

var oldstoreKey = struct{}{}

func loadSandboxConfigFromOldStore(ctx context.Context, sid string) (*SandboxConfig, context.Context, error) {
	var config SandboxConfig
	// We're bootstrapping
	vcStore, err := store.NewVCSandboxStore(ctx, sid)
	if err != nil {
		return nil, ctx, err
	}

	if err := vcStore.Load(store.Configuration, &config); err != nil {
		return nil, ctx, err
	}

	return &config, context.WithValue(ctx, oldstoreKey, true), nil
}

func useOldStore(ctx context.Context) bool {
	v := ctx.Value(oldstoreKey)
	return v != nil
}
