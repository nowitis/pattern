// Copyright (C) 2023 - Perceval Faramaz
// Portions Copyright (C) 2022, 2023 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"

	"github.com/tillitis/tkeyclient"
)

var (
	cmdGetNameVersion = appCmd{0x01, "cmdGetNameVersion", tkeyclient.CmdLen1}
	rspGetNameVersion = appCmd{0x02, "rspGetNameVersion", tkeyclient.CmdLen32}
	cmdSetPattern     = appCmd{0x03, "cmdSetPattern", tkeyclient.CmdLen128}
	rspSetPattern     = appCmd{0x04, "rspSetPattern", tkeyclient.CmdLen4}
	cmdGetPattern     = appCmd{0x05, "cmdGetPattern", tkeyclient.CmdLen1}
	rspGetPattern     = appCmd{0x06, "rspGetPattern", tkeyclient.CmdLen128}
	cmdExecute        = appCmd{0x07, "cmdExecute", tkeyclient.CmdLen1}
	rspExecute        = appCmd{0x08, "rspExecute", tkeyclient.CmdLen1}
)

type appCmd struct {
	code   byte
	name   string
	cmdLen tkeyclient.CmdLen
}

func (c appCmd) Code() byte {
	return c.code
}

func (c appCmd) CmdLen() tkeyclient.CmdLen {
	return c.cmdLen
}

func (c appCmd) Endpoint() tkeyclient.Endpoint {
	return tkeyclient.DestApp
}

func (c appCmd) String() string {
	return c.name
}

type PatternBlinker struct {
	tk *tkeyclient.TillitisKey // A connection to a TKey
}

// New allocates a struct for communicating with the random app
// running on the TKey. You're expected to pass an existing connection
// to it, so use it like this:
//
//	tk := tkeyclient.New()
//	err := tk.Connect(port)
//	blinker := New(tk)
func New(tk *tkeyclient.TillitisKey) PatternBlinker {
	var blinker PatternBlinker

	blinker.tk = tk

	return blinker
}

// Close closes the connection to the TKey
func (p PatternBlinker) Close() error {
	if err := p.tk.Close(); err != nil {
		return fmt.Errorf("tk.Close: %w", err)
	}
	return nil
}

// GetAppNameVersion gets the name and version of the running app in
// the same style as the stick itself.
func (p PatternBlinker) GetAppNameVersion() (*tkeyclient.NameVersion, error) {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdGetNameVersion, id)
	if err != nil {
		return nil, fmt.Errorf("NewFrameBuf: %w", err)
	}

	tkeyclient.Dump("GetAppNameVersion tx", tx)
	if err = p.tk.Write(tx); err != nil {
		return nil, fmt.Errorf("Write: %w", err)
	}

	err = p.tk.SetReadTimeout(2)
	if err != nil {
		return nil, fmt.Errorf("SetReadTimeout: %w", err)
	}

	rx, _, err := p.tk.ReadFrame(rspGetNameVersion, id)
	if err != nil {
		return nil, fmt.Errorf("ReadFrame: %w", err)
	}

	err = p.tk.SetReadTimeout(0)
	if err != nil {
		return nil, fmt.Errorf("SetReadTimeout: %w", err)
	}

	nameVer := &tkeyclient.NameVersion{}
	nameVer.Unpack(rx[2:])

	return nameVer, nil
}

// SetPattern loads a LED pattern on the key.
func (p PatternBlinker) SetPattern(pattern []byte, objectCount int) error {
	var offset int
	var err error

	data := make([]byte, len(pattern)+1)
	data[0] = (uint8)(objectCount)
	_ = copy(data[1:], pattern)

	for nsent := 0; offset < len(data); offset += nsent {
		nsent, err = p.sendChunk(cmdSetPattern, rspSetPattern, data[offset:])
		if err != nil {
			return fmt.Errorf("SetPattern: %w", err)
		}
	}
	if offset > len(data) {
		return fmt.Errorf("transmitted more than expected")
	}

	return nil
}

func (p PatternBlinker) sendChunk(cmd appCmd, rsp appCmd, content []byte) (int, error) {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmd, id)
	if err != nil {
		return 0, fmt.Errorf("NewFrameBuf: %w", err)
	}

	payload := make([]byte, cmd.CmdLen().Bytelen()-1)
	copied := copy(payload, content)

	// Add padding if not filling the payload buffer.
	if copied < len(payload) {
		padding := make([]byte, len(payload)-copied)
		copy(payload[copied:], padding)
	}

	copy(tx[2:], payload)

	tkeyclient.Dump("sendChunk tx", tx)
	if err = p.tk.Write(tx); err != nil {
		return 0, fmt.Errorf("Write: %w", err)
	}

	// Wait for reply
	rx, _, err := p.tk.ReadFrame(rsp, id)
	if err != nil {
		return 0, fmt.Errorf("ReadFrame: %w", err)
	}

	if rx[2] != tkeyclient.StatusOK {
		return 0, fmt.Errorf("putSendChunk NOK")
	}

	return copied, nil
}

// Execute starts the LED pattern, blocking during done.
func (p PatternBlinker) Execute() error {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdExecute, id)
	if err != nil {
		return fmt.Errorf("NewFrameBuf: %w", err)
	}

	tkeyclient.Dump("Execute tx", tx)
	if err = p.tk.Write(tx); err != nil {
		return fmt.Errorf("Write: %w", err)
	}

	rx, _, err := p.tk.ReadFrame(rspExecute, id)
	tkeyclient.Dump("Execute rx", rx)
	if err != nil {
		return fmt.Errorf("ReadFrame: %w", err)
	}

	return nil
}

// GetPattern retrieves the LED pattern from the key.
func (p PatternBlinker) GetPattern(objectSize int) ([]byte, error) {
	id := 2
	var expectedObjects int
	var payload []byte

	for nreceivedBytes := -1; nreceivedBytes < (expectedObjects * objectSize); {
		tx, err := tkeyclient.NewFrameBuf(cmdGetPattern, id)
		if err != nil {
			return nil, fmt.Errorf("NewFrameBuf: %w", err)
		}

		tkeyclient.Dump("GetPattern tx", tx)
		if err = p.tk.Write(tx); err != nil {
			return nil, fmt.Errorf("Write: %w", err)
		}

		rx, _, err := p.tk.ReadFrame(rspGetPattern, id)
		if err != nil {
			return nil, fmt.Errorf("ReadFrame: %w", err)
		}

		skip := 0
		if nreceivedBytes == -1 {
			expectedObjects = (int)((uint8)(rx[3]))
			payload = make([]byte, expectedObjects * objectSize)
			skip = 1
			nreceivedBytes = 0
		}

		nreceivedBytes += copy(payload[nreceivedBytes:], rx[3+skip:])
	}

	return payload, nil
}
