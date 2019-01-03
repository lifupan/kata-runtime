// Copyright (c) 2016 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package virtcontainers

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/kata-containers/runtime/virtcontainers/pkg/mock"
	"github.com/kata-containers/runtime/virtcontainers/pkg/types"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

const (
	containerID = "1"
)

var sandboxAnnotations = map[string]string{
	"sandbox.foo":   "sandbox.bar",
	"sandbox.hello": "sandbox.world",
}

var containerAnnotations = map[string]string{
	"container.foo":   "container.bar",
	"container.hello": "container.world",
}

func newBasicTestCmd() Cmd {
	envs := []EnvVar{
		{
			Var:   "PATH",
			Value: "/bin:/usr/bin:/sbin:/usr/sbin",
		},
	}

	cmd := Cmd{
		Args:    strings.Split("/bin/sh", " "),
		Envs:    envs,
		WorkDir: "/",
	}

	return cmd
}

func newTestSandboxConfigNoop() SandboxConfig {
	// Define the container command and bundle.
	container := ContainerConfig{
		ID:          containerID,
		RootFs:      filepath.Join(testDir, testBundle),
		Cmd:         newBasicTestCmd(),
		Annotations: containerAnnotations,
	}

	// Sets the hypervisor configuration.
	hypervisorConfig := HypervisorConfig{
		KernelPath:     filepath.Join(testDir, testKernel),
		ImagePath:      filepath.Join(testDir, testImage),
		HypervisorPath: filepath.Join(testDir, testHypervisor),
	}

	sandboxConfig := SandboxConfig{
		ID:               testSandboxID,
		HypervisorType:   MockHypervisor,
		HypervisorConfig: hypervisorConfig,

		AgentType: NoopAgentType,

		Containers: []ContainerConfig{container},

		Annotations: sandboxAnnotations,
	}

	return sandboxConfig
}

func newTestSandboxConfigHyperstartAgent() SandboxConfig {
	// Define the container command and bundle.
	container := ContainerConfig{
		ID:          containerID,
		RootFs:      filepath.Join(testDir, testBundle),
		Cmd:         newBasicTestCmd(),
		Annotations: containerAnnotations,
	}

	// Sets the hypervisor configuration.
	hypervisorConfig := HypervisorConfig{
		KernelPath:     filepath.Join(testDir, testKernel),
		ImagePath:      filepath.Join(testDir, testImage),
		HypervisorPath: filepath.Join(testDir, testHypervisor),
	}

	agentConfig := HyperConfig{
		SockCtlName: testHyperstartCtlSocket,
		SockTtyName: testHyperstartTtySocket,
	}

	sandboxConfig := SandboxConfig{
		ID:               testSandboxID,
		HypervisorType:   MockHypervisor,
		HypervisorConfig: hypervisorConfig,

		AgentType:   HyperstartAgent,
		AgentConfig: agentConfig,

		Containers:  []ContainerConfig{container},
		Annotations: sandboxAnnotations,
	}

	return sandboxConfig
}

func newTestSandboxConfigHyperstartAgentDefaultNetwork() SandboxConfig {
	// Define the container command and bundle.
	container := ContainerConfig{
		ID:          containerID,
		RootFs:      filepath.Join(testDir, testBundle),
		Cmd:         newBasicTestCmd(),
		Annotations: containerAnnotations,
	}

	// Sets the hypervisor configuration.
	hypervisorConfig := HypervisorConfig{
		KernelPath:     filepath.Join(testDir, testKernel),
		ImagePath:      filepath.Join(testDir, testImage),
		HypervisorPath: filepath.Join(testDir, testHypervisor),
	}

	agentConfig := HyperConfig{
		SockCtlName: testHyperstartCtlSocket,
		SockTtyName: testHyperstartTtySocket,
	}

	netConfig := NetworkConfig{}

	sandboxConfig := SandboxConfig{
		ID: testSandboxID,

		HypervisorType:   MockHypervisor,
		HypervisorConfig: hypervisorConfig,

		AgentType:   HyperstartAgent,
		AgentConfig: agentConfig,

		NetworkModel:  DefaultNetworkModel,
		NetworkConfig: netConfig,

		Containers:  []ContainerConfig{container},
		Annotations: sandboxAnnotations,
	}

	return sandboxConfig
}

func newTestSandboxConfigKataAgent() SandboxConfig {
	// Sets the hypervisor configuration.
	hypervisorConfig := HypervisorConfig{
		KernelPath:     filepath.Join(testDir, testKernel),
		ImagePath:      filepath.Join(testDir, testImage),
		HypervisorPath: filepath.Join(testDir, testHypervisor),
	}

	sandboxConfig := SandboxConfig{
		ID:               testSandboxID,
		HypervisorType:   MockHypervisor,
		HypervisorConfig: hypervisorConfig,

		AgentType: KataContainersAgent,

		Annotations: sandboxAnnotations,
	}

	return sandboxConfig
}

func TestCreateSandboxNoopAgentSuccessful(t *testing.T) {
	cleanUp()

	config := newTestSandboxConfigNoop()

	p, err := CreateSandbox(context.Background(), config, nil)
	if p == nil || err != nil {
		t.Fatal(err)
	}

	sandboxDir := filepath.Join(configStoragePath, p.ID())
	_, err = os.Stat(sandboxDir)
	if err != nil {
		t.Fatal(err)
	}
}

var testCCProxySockPathTempl = "%s/cc-proxy-test.sock"
var testCCProxyURLUnixScheme = "unix://"

func testGenerateCCProxySockDir() (string, error) {
	dir, err := ioutil.TempDir("", "cc-proxy-test")
	if err != nil {
		return "", err
	}

	return dir, nil
}

func TestCreateSandboxHyperstartAgentSuccessful(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip(testDisabledAsNonRoot)
	}

	cleanUp()

	config := newTestSandboxConfigHyperstartAgent()

	sockDir, err := testGenerateCCProxySockDir()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(sockDir)

	testCCProxySockPath := fmt.Sprintf(testCCProxySockPathTempl, sockDir)
	noopProxyURL = testCCProxyURLUnixScheme + testCCProxySockPath
	proxy := mock.NewCCProxyMock(t, testCCProxySockPath)
	proxy.Start()
	defer proxy.Stop()

	p, err := CreateSandbox(context.Background(), config, nil)
	if p == nil || err != nil {
		t.Fatal(err)
	}

	sandboxDir := filepath.Join(configStoragePath, p.ID())
	_, err = os.Stat(sandboxDir)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateSandboxKataAgentSuccessful(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip(testDisabledAsNonRoot)
	}

	cleanUp()

	config := newTestSandboxConfigKataAgent()

	sockDir, err := testGenerateKataProxySockDir()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(sockDir)

	testKataProxyURL := fmt.Sprintf(testKataProxyURLTempl, sockDir)
	noopProxyURL = testKataProxyURL

	impl := &gRPCProxy{}

	kataProxyMock := mock.ProxyGRPCMock{
		GRPCImplementer: impl,
		GRPCRegister:    gRPCRegister,
	}
	if err := kataProxyMock.Start(testKataProxyURL); err != nil {
		t.Fatal(err)
	}
	defer kataProxyMock.Stop()

	p, err := CreateSandbox(context.Background(), config, nil)
	if p == nil || err != nil {
		t.Fatal(err)
	}

	sandboxDir := filepath.Join(configStoragePath, p.ID())
	_, err = os.Stat(sandboxDir)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateSandboxFailing(t *testing.T) {
	cleanUp()

	config := SandboxConfig{}

	p, err := CreateSandbox(context.Background(), config, nil)
	if p.(*Sandbox) != nil || err == nil {
		t.Fatal()
	}
}

func TestDeleteSandboxNoopAgentSuccessful(t *testing.T) {
	cleanUp()

	ctx := context.Background()
	config := newTestSandboxConfigNoop()

	p, err := CreateSandbox(ctx, config, nil)
	if p == nil || err != nil {
		t.Fatal(err)
	}

	sandboxDir := filepath.Join(configStoragePath, p.ID())
	_, err = os.Stat(sandboxDir)
	if err != nil {
		t.Fatal(err)
	}

	err = p.Delete()
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(sandboxDir)
	if err == nil {
		t.Fatal()
	}
}

func TestDeleteSandboxHyperstartAgentSuccessful(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip(testDisabledAsNonRoot)
	}

	cleanUp()

	config := newTestSandboxConfigHyperstartAgent()

	sockDir, err := testGenerateCCProxySockDir()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(sockDir)

	testCCProxySockPath := fmt.Sprintf(testCCProxySockPathTempl, sockDir)
	noopProxyURL = testCCProxyURLUnixScheme + testCCProxySockPath
	proxy := mock.NewCCProxyMock(t, testCCProxySockPath)
	proxy.Start()
	defer proxy.Stop()

	ctx := context.Background()

	p, err := CreateSandbox(ctx, config, nil)
	if p == nil || err != nil {
		t.Fatal(err)
	}

	sandboxDir := filepath.Join(configStoragePath, p.ID())
	_, err = os.Stat(sandboxDir)
	if err != nil {
		t.Fatal(err)
	}

	err = p.Delete()
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(sandboxDir)
	if err == nil {
		t.Fatal(err)
	}
}

func TestDeleteSandboxKataAgentSuccessful(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip(testDisabledAsNonRoot)
	}

	cleanUp()

	config := newTestSandboxConfigKataAgent()

	sockDir, err := testGenerateKataProxySockDir()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(sockDir)

	testKataProxyURL := fmt.Sprintf(testKataProxyURLTempl, sockDir)
	noopProxyURL = testKataProxyURL

	impl := &gRPCProxy{}

	kataProxyMock := mock.ProxyGRPCMock{
		GRPCImplementer: impl,
		GRPCRegister:    gRPCRegister,
	}
	if err := kataProxyMock.Start(testKataProxyURL); err != nil {
		t.Fatal(err)
	}
	defer kataProxyMock.Stop()

	ctx := context.Background()
	p, err := CreateSandbox(ctx, config, nil)
	if p == nil || err != nil {
		t.Fatal(err)
	}

	sandboxDir := filepath.Join(configStoragePath, p.ID())
	_, err = os.Stat(sandboxDir)
	if err != nil {
		t.Fatal(err)
	}

	err = p.Delete()
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(sandboxDir)
	if err == nil {
		t.Fatal(err)
	}
}

func TestStartSandboxNoopAgentSuccessful(t *testing.T) {
	cleanUp()

	config := newTestSandboxConfigNoop()

	p, _, err := createAndStartSandbox(context.Background(), config)
	if p == nil || err != nil {
		t.Fatal(err)
	}
}

func TestStartSandboxHyperstartAgentSuccessful(t *testing.T) {
	cleanUp()

	if os.Geteuid() != 0 {
		t.Skip(testDisabledAsNonRoot)
	}

	config := newTestSandboxConfigHyperstartAgent()

	sockDir, err := testGenerateCCProxySockDir()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(sockDir)

	testCCProxySockPath := fmt.Sprintf(testCCProxySockPathTempl, sockDir)
	noopProxyURL = testCCProxyURLUnixScheme + testCCProxySockPath
	proxy := mock.NewCCProxyMock(t, testCCProxySockPath)
	proxy.Start()
	defer proxy.Stop()

	hyperConfig := config.AgentConfig.(HyperConfig)
	config.AgentConfig = hyperConfig

	ctx := context.Background()
	p, _, err := createAndStartSandbox(ctx, config)
	if p == nil || err != nil {
		t.Fatal(err)
	}

	pImpl, ok := p.(*Sandbox)
	assert.True(t, ok)

	bindUnmountAllRootfs(ctx, defaultSharedDir, pImpl)
}

func TestStartSandboxKataAgentSuccessful(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip(testDisabledAsNonRoot)
	}

	cleanUp()

	config := newTestSandboxConfigKataAgent()

	sockDir, err := testGenerateKataProxySockDir()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(sockDir)

	testKataProxyURL := fmt.Sprintf(testKataProxyURLTempl, sockDir)
	noopProxyURL = testKataProxyURL

	impl := &gRPCProxy{}

	kataProxyMock := mock.ProxyGRPCMock{
		GRPCImplementer: impl,
		GRPCRegister:    gRPCRegister,
	}
	if err := kataProxyMock.Start(testKataProxyURL); err != nil {
		t.Fatal(err)
	}
	defer kataProxyMock.Stop()

	ctx := context.Background()
	p, _, err := createAndStartSandbox(ctx, config)
	if p == nil || err != nil {
		t.Fatal(err)
	}

	pImpl, ok := p.(*Sandbox)
	assert.True(t, ok)

	bindUnmountAllRootfs(ctx, defaultSharedDir, pImpl)
}

func TestListSandboxSuccessful(t *testing.T) {
	cleanUp()

	os.RemoveAll(configStoragePath)

	config := newTestSandboxConfigNoop()

	ctx := context.Background()
	p, err := CreateSandbox(ctx, config, nil)
	if p == nil || err != nil {
		t.Fatal(err)
	}

	_, err = ListSandbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
}

func TestListSandboxNoSandboxDirectory(t *testing.T) {
	cleanUp()

	os.RemoveAll(configStoragePath)

	_, err := ListSandbox(context.Background())
	if err != nil {
		t.Fatal(fmt.Sprintf("unexpected ListSandbox error from non-existent sandbox directory: %v", err))
	}
}

func TestStatusSandboxSuccessfulStateReady(t *testing.T) {
	cleanUp()

	config := newTestSandboxConfigNoop()
	hypervisorConfig := HypervisorConfig{
		KernelPath:        filepath.Join(testDir, testKernel),
		ImagePath:         filepath.Join(testDir, testImage),
		HypervisorPath:    filepath.Join(testDir, testHypervisor),
		NumVCPUs:          defaultVCPUs,
		MemorySize:        defaultMemSzMiB,
		DefaultBridges:    defaultBridges,
		BlockDeviceDriver: defaultBlockDriver,
		DefaultMaxVCPUs:   defaultMaxQemuVCPUs,
		Msize9p:           defaultMsize9p,
	}

	expectedStatus := SandboxStatus{
		ID: testSandboxID,
		State: State{
			State: StateReady,
		},
		Hypervisor:       MockHypervisor,
		HypervisorConfig: hypervisorConfig,
		Agent:            NoopAgentType,
		Annotations:      sandboxAnnotations,
		ContainersStatus: []ContainerStatus{
			{
				ID: containerID,
				State: State{
					State: StateReady,
				},
				PID:         0,
				RootFs:      filepath.Join(testDir, testBundle),
				Annotations: containerAnnotations,
			},
		},
	}

	ctx := context.Background()
	p, err := CreateSandbox(ctx, config, nil)
	if p == nil || err != nil {
		t.Fatal(err)
	}

	status, err := StatusSandbox(ctx, p.ID())
	if err != nil {
		t.Fatal(err)
	}

	// Copy the start time as we can't pretend we know what that
	// value will be.
	expectedStatus.ContainersStatus[0].StartTime = status.ContainersStatus[0].StartTime

	if reflect.DeepEqual(status, expectedStatus) == false {
		t.Fatalf("Got sandbox status %v\n expecting %v", status, expectedStatus)
	}
}

func TestStatusSandboxSuccessfulStateRunning(t *testing.T) {
	cleanUp()

	config := newTestSandboxConfigNoop()
	hypervisorConfig := HypervisorConfig{
		KernelPath:        filepath.Join(testDir, testKernel),
		ImagePath:         filepath.Join(testDir, testImage),
		HypervisorPath:    filepath.Join(testDir, testHypervisor),
		NumVCPUs:          defaultVCPUs,
		MemorySize:        defaultMemSzMiB,
		DefaultBridges:    defaultBridges,
		BlockDeviceDriver: defaultBlockDriver,
		DefaultMaxVCPUs:   defaultMaxQemuVCPUs,
		Msize9p:           defaultMsize9p,
	}

	expectedStatus := SandboxStatus{
		ID: testSandboxID,
		State: State{
			State: StateRunning,
		},
		Hypervisor:       MockHypervisor,
		HypervisorConfig: hypervisorConfig,
		Agent:            NoopAgentType,
		Annotations:      sandboxAnnotations,
		ContainersStatus: []ContainerStatus{
			{
				ID: containerID,
				State: State{
					State: StateRunning,
				},
				PID:         0,
				RootFs:      filepath.Join(testDir, testBundle),
				Annotations: containerAnnotations,
			},
		},
	}

	ctx := context.Background()
	p, err := CreateSandbox(ctx, config, nil)
	if p == nil || err != nil {
		t.Fatal(err)
	}

	err = p.Start()
	if err != nil {
		t.Fatal(err)
	}

	status, err := StatusSandbox(ctx, p.ID())
	if err != nil {
		t.Fatal(err)
	}

	// Copy the start time as we can't pretend we know what that
	// value will be.
	expectedStatus.ContainersStatus[0].StartTime = status.ContainersStatus[0].StartTime

	if reflect.DeepEqual(status, expectedStatus) == false {
		t.Fatalf("Got sandbox status %v\n expecting %v", status, expectedStatus)
	}
}

func TestStatusSandboxFailingFetchSandboxConfig(t *testing.T) {
	cleanUp()

	config := newTestSandboxConfigNoop()

	ctx := context.Background()
	p, err := CreateSandbox(ctx, config, nil)
	if p == nil || err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(configStoragePath, p.ID())
	os.RemoveAll(path)
	globalSandboxList.removeSandbox(p.ID())

	_, err = StatusSandbox(ctx, p.ID())
	if err == nil {
		t.Fatal()
	}
}

func TestStatusPodSandboxFailingFetchSandboxState(t *testing.T) {
	cleanUp()

	config := newTestSandboxConfigNoop()

	ctx := context.Background()
	p, err := CreateSandbox(ctx, config, nil)
	if p == nil || err != nil {
		t.Fatal(err)
	}

	pImpl, ok := p.(*Sandbox)
	assert.True(t, ok)

	os.RemoveAll(pImpl.configPath)
	globalSandboxList.removeSandbox(p.ID())

	_, err = StatusSandbox(ctx, p.ID())
	if err == nil {
		t.Fatal()
	}
}

func newTestContainerConfigNoop(contID string) ContainerConfig {
	// Define the container command and bundle.
	container := ContainerConfig{
		ID:          contID,
		RootFs:      filepath.Join(testDir, testBundle),
		Cmd:         newBasicTestCmd(),
		Annotations: containerAnnotations,
	}

	return container
}

/*
 * Benchmarks
 */

func createNewSandboxConfig(hType HypervisorType, aType AgentType, aConfig interface{}, netModel NetworkModel) SandboxConfig {
	hypervisorConfig := HypervisorConfig{
		KernelPath:     "/usr/share/kata-containers/vmlinux.container",
		ImagePath:      "/usr/share/kata-containers/kata-containers.img",
		HypervisorPath: "/usr/bin/qemu-system-x86_64",
	}

	netConfig := NetworkConfig{}

	return SandboxConfig{
		ID:               testSandboxID,
		HypervisorType:   hType,
		HypervisorConfig: hypervisorConfig,

		AgentType:   aType,
		AgentConfig: aConfig,

		NetworkModel:  netModel,
		NetworkConfig: netConfig,
	}
}

func createNewContainerConfigs(numOfContainers int) []ContainerConfig {
	var contConfigs []ContainerConfig

	envs := []EnvVar{
		{
			Var:   "PATH",
			Value: "/bin:/usr/bin:/sbin:/usr/sbin",
		},
	}

	cmd := Cmd{
		Args:    strings.Split("/bin/ps -A", " "),
		Envs:    envs,
		WorkDir: "/",
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if ok == false {
		return nil
	}

	rootFs := filepath.Dir(thisFile) + "/utils/supportfiles/bundles/busybox/"

	for i := 0; i < numOfContainers; i++ {
		contConfig := ContainerConfig{
			ID:     fmt.Sprintf("%d", i),
			RootFs: rootFs,
			Cmd:    cmd,
		}

		contConfigs = append(contConfigs, contConfig)
	}

	return contConfigs
}

// createAndStartSandbox handles the common test operation of creating and
// starting a sandbox.
func createAndStartSandbox(ctx context.Context, config SandboxConfig) (sandbox VCSandbox, sandboxDir string,
	err error) {

	// Create sandbox
	sandbox, err = CreateSandbox(ctx, config, nil)
	if sandbox == nil || err != nil {
		return nil, "", err
	}

	sandboxDir = filepath.Join(configStoragePath, sandbox.ID())
	_, err = os.Stat(sandboxDir)
	if err != nil {
		return nil, "", err
	}

	// Start sandbox
	err = sandbox.Start()
	if err != nil {
		return nil, "", err
	}

	return sandbox, sandboxDir, nil
}

func createStartStopDeleteSandbox(b *testing.B, sandboxConfig SandboxConfig) {
	ctx := context.Background()

	p, _, err := createAndStartSandbox(ctx, sandboxConfig)
	if p == nil || err != nil {
		b.Fatalf("Could not create and start sandbox: %s", err)
	}

	// Stop sandbox
	err = p.Stop()
	if err != nil {
		b.Fatalf("Could not stop sandbox: %s", err)
	}

	// Delete sandbox
	err = p.Delete()
	if err != nil {
		b.Fatalf("Could not delete sandbox: %s", err)
	}
}

func createStartStopDeleteContainers(b *testing.B, sandboxConfig SandboxConfig, contConfigs []ContainerConfig) {
	ctx := context.Background()

	// Create sandbox
	p, err := CreateSandbox(ctx, sandboxConfig, nil)
	if err != nil {
		b.Fatalf("Could not create sandbox: %s", err)
	}

	// Start sandbox
	err = p.Start()
	if err != nil {
		b.Fatalf("Could not start sandbox: %s", err)
	}

	// Create containers
	for _, contConfig := range contConfigs {
		_, err := p.CreateContainer(contConfig)
		if err != nil {
			b.Fatalf("Could not create container %s: %s", contConfig.ID, err)
		}
	}

	// Start containers
	for _, contConfig := range contConfigs {
		_, err := p.StartContainer(contConfig.ID)
		if err != nil {
			b.Fatalf("Could not start container %s: %s", contConfig.ID, err)
		}
	}

	// Stop containers
	for _, contConfig := range contConfigs {
		_, err := p.StopContainer(contConfig.ID)
		if err != nil {
			b.Fatalf("Could not stop container %s: %s", contConfig.ID, err)
		}
	}

	// Delete containers
	for _, contConfig := range contConfigs {
		_, err := p.DeleteContainer(contConfig.ID)
		if err != nil {
			b.Fatalf("Could not delete container %s: %s", contConfig.ID, err)
		}
	}

	// Stop sandbox
	err = p.Stop()
	if err != nil {
		b.Fatalf("Could not stop sandbox: %s", err)
	}

	// Delete sandbox
	err = p.Delete()
	if err != nil {
		b.Fatalf("Could not delete sandbox: %s", err)
	}
}

func BenchmarkCreateStartStopDeleteSandboxQemuHypervisorHyperstartAgentNetworkNoop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sandboxConfig := createNewSandboxConfig(QemuHypervisor, HyperstartAgent, HyperConfig{}, NoopNetworkModel)

		sockDir, err := testGenerateCCProxySockDir()
		if err != nil {
			b.Fatal(err)
		}
		defer os.RemoveAll(sockDir)

		var t testing.T
		testCCProxySockPath := fmt.Sprintf(testCCProxySockPathTempl, sockDir)
		noopProxyURL = testCCProxyURLUnixScheme + testCCProxySockPath
		proxy := mock.NewCCProxyMock(&t, testCCProxySockPath)
		proxy.Start()
		defer proxy.Stop()

		createStartStopDeleteSandbox(b, sandboxConfig)
	}
}

func BenchmarkCreateStartStopDeleteSandboxQemuHypervisorNoopAgentNetworkNoop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sandboxConfig := createNewSandboxConfig(QemuHypervisor, NoopAgentType, nil, NoopNetworkModel)
		createStartStopDeleteSandbox(b, sandboxConfig)
	}
}

func BenchmarkCreateStartStopDeleteSandboxMockHypervisorNoopAgentNetworkNoop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sandboxConfig := createNewSandboxConfig(MockHypervisor, NoopAgentType, nil, NoopNetworkModel)
		createStartStopDeleteSandbox(b, sandboxConfig)
	}
}

func BenchmarkStartStop1ContainerQemuHypervisorHyperstartAgentNetworkNoop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sandboxConfig := createNewSandboxConfig(QemuHypervisor, HyperstartAgent, HyperConfig{}, NoopNetworkModel)
		contConfigs := createNewContainerConfigs(1)

		sockDir, err := testGenerateCCProxySockDir()
		if err != nil {
			b.Fatal(err)
		}
		defer os.RemoveAll(sockDir)

		var t testing.T
		testCCProxySockPath := fmt.Sprintf(testCCProxySockPathTempl, sockDir)
		noopProxyURL = testCCProxyURLUnixScheme + testCCProxySockPath
		proxy := mock.NewCCProxyMock(&t, testCCProxySockPath)
		proxy.Start()
		defer proxy.Stop()

		createStartStopDeleteContainers(b, sandboxConfig, contConfigs)
	}
}

func BenchmarkStartStop10ContainerQemuHypervisorHyperstartAgentNetworkNoop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sandboxConfig := createNewSandboxConfig(QemuHypervisor, HyperstartAgent, HyperConfig{}, NoopNetworkModel)
		contConfigs := createNewContainerConfigs(10)

		sockDir, err := testGenerateCCProxySockDir()
		if err != nil {
			b.Fatal(err)
		}
		defer os.RemoveAll(sockDir)

		var t testing.T
		testCCProxySockPath := fmt.Sprintf(testCCProxySockPathTempl, sockDir)
		noopProxyURL = testCCProxyURLUnixScheme + testCCProxySockPath
		proxy := mock.NewCCProxyMock(&t, testCCProxySockPath)
		proxy.Start()
		defer proxy.Stop()

		createStartStopDeleteContainers(b, sandboxConfig, contConfigs)
	}
}

func TestFetchSandbox(t *testing.T) {
	cleanUp()

	config := newTestSandboxConfigNoop()

	ctx := context.Background()

	s, err := CreateSandbox(ctx, config, nil)
	if s == nil || err != nil {
		t.Fatal(err)
	}

	fetched, err := FetchSandbox(ctx, s.ID())
	assert.Nil(t, err, "%v", err)
	assert.True(t, fetched != s, "fetched stateless sandboxes should not match")
}

func TestFetchStatefulSandbox(t *testing.T) {
	cleanUp()

	config := newTestSandboxConfigNoop()

	config.Stateful = true

	ctx := context.Background()

	s, err := CreateSandbox(ctx, config, nil)
	if s == nil || err != nil {
		t.Fatal(err)
	}

	fetched, err := FetchSandbox(ctx, s.ID())
	assert.Nil(t, err, "%v", err)
	assert.Equal(t, fetched, s, "fetched stateful sandboxed should match")
}

func TestFetchNonExistingSandbox(t *testing.T) {
	cleanUp()

	_, err := FetchSandbox(context.Background(), "some-non-existing-sandbox-name")
	assert.NotNil(t, err, "fetch non-existing sandbox should fail")
}

func TestReleaseSandbox(t *testing.T) {
	cleanUp()

	config := newTestSandboxConfigNoop()

	s, err := CreateSandbox(context.Background(), config, nil)
	if s == nil || err != nil {
		t.Fatal(err)
	}
	err = s.Release()
	assert.Nil(t, err, "sandbox release failed: %v", err)
}
