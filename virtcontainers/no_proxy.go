// Copyright (c) 2017 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package virtcontainers

import (
	"bufio"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"net"
)

// This is the no proxy implementation of the proxy interface. This
// is a generic implementation for any case (basically any agent),
// where no actual proxy is needed. This happens when the combination
// of the VM and the agent can handle multiple connections without
// additional component to handle the multiplexing. Both the runtime
// and the shim will connect to the agent through the VM, bypassing
// the proxy model.
// That's why this implementation is very generic, and all it does
// is to provide both shim and runtime the correct URL to connect
// directly to the VM.
type noProxy struct {
}

// start is noProxy start implementation for proxy interface.
func (p *noProxy) start(params proxyParams) (int, string, error) {
	if params.logger == nil {
		return -1, "", fmt.Errorf("proxy logger is not set")
	}

	params.logger.Debug("---------------------No proxy started because of no-proxy implementation")
	
	params.logger.Debugf("========================watchconsole on %s", params.consoleURL)

	if params.agentURL == "" {
		return -1, "", fmt.Errorf("AgentURL cannot be empty")
	}
	err := p.watchConsole(buildinProxyConsoleProto, params.consoleURL, params.logger)
	if err != nil {
		return -1, "", err
	}
	return 0, params.agentURL, nil
}

// stop is noProxy stop implementation for proxy interface.
func (p *noProxy) stop(pid int) error {
	return nil
}

// The noproxy doesn't need to watch the vm console, thus return false always.
func (p *noProxy) consoleWatched() bool {
       return false
}

func (p *noProxy) watchConsole(proto, console string, logger *logrus.Entry) (err error) {
	var (
		scanner *bufio.Scanner
		conn    net.Conn
	)

	switch proto {
	case consoleProtoUnix:
		conn, err = net.Dial("unix", console)
		if err != nil {
			return err
		}
		// TODO: add pty console support for kvmtools
	case consoleProtoPty:
		fallthrough
	default:
		return fmt.Errorf("unknown console proto %s", proto)
	}

	logger.Infof("========================watchconsole on %s", console)
//	p.conn = conn

	go func() {
		scanner = bufio.NewScanner(conn)
		for scanner.Scan() {
			logger.WithFields(logrus.Fields{
//				"sandbox":   p.sandboxID,
				"vmconsole": scanner.Text(),
			}).Debug("reading guest console")
		}

		if err := scanner.Err(); err != nil {
			if err == io.EOF {
				logger.Info("console watcher quits")
			} else {
				logger.WithError(err).WithFields(logrus.Fields{
					"console-protocol": proto,
					"console-socket":   console,
				}).Error("Failed to read agent logs")
			}
		}
	}()

	return nil
}
