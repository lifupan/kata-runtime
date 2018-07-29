// Copyright (c) 2018 HyperHQ Inc.
//
// SPDX-License-Identifier: Apache-2.0
//

package kata

type Exec struct {
	Id        string
	Container *Container
	Cmds      []string
	Terminal  bool
	ExitCode  uint8

	finChan   chan bool
}
