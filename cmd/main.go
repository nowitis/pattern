// Copyright (C) 2023 - Perceval Faramaz
// Portions Copyright (C) 2022, 2023 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

package main

/*
#include "c_shim.h"
*/
import "C"

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"unsafe"
	"strings"
	"bytes"

	"github.com/spf13/pflag"
	"github.com/nowitis/pattern/internal/util"
	"github.com/tillitis/tkeyclient"
)

// nolint:typecheck // Avoid lint error when the embedding file is missing.
// Makefile copies the built app here ./app.bin
//
//go:embed app.bin
var appBinary []byte

const (
	wantFWName0  = "tk1 "
	wantFWName1  = "mkdf"
	wantAppName0 = "tk1 "
	wantAppName1 = "ptrn"
)

var le = log.New(os.Stderr, "", 0)

func main() {
	var devPath string
	var speed int
	var patternString string
	var helpOnly bool
	pflag.CommandLine.SortFlags = false
	pflag.StringVar(&devPath, "port", "",
		"Set serial port device `PATH`. If this is not passed, auto-detection will be attempted.")
	pflag.IntVar(&speed, "speed", tkeyclient.SerialSpeed,
		"Set serial port speed in `BPS` (bits per second).")
	pflag.StringVar(&patternString, "pattern", "",
		"The morse pattern, described from dashes and dots.")
	pflag.BoolVar(&helpOnly, "help", false, "Output this help.")
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, `runpattern is a client app for the pattern app, a mock app that 
makes the Tillitis TKey LED blink following a pattern defined in morse. This program embeds the 
app binary, which it loads onto the USB stick and starts.

Usage:

%s`,
			pflag.CommandLine.FlagUsagesWrapped(80))
	}
	pflag.Parse()

	if helpOnly {
		pflag.Usage()
		os.Exit(0)
	}

	if patternString == "" {
		le.Printf("Please set LED pattern with --pattern\n")
		pflag.Usage()
		os.Exit(2)
	}

	pattern, patternLength := GetLedData(patternString)
	if pattern == nil {
		le.Printf("Please set LED pattern using only dots and dashes\n")
		pflag.Usage()
		os.Exit(2)
	}

	if devPath == "" {
		var err error
		devPath, err = util.DetectSerialPort(true)
		if err != nil {
			os.Exit(1)
		}
	}

	tkeyclient.SilenceLogging()

	tk := tkeyclient.New()
	le.Printf("Connecting to device on serial port %s...\n", devPath)
	if err := tk.Connect(devPath, tkeyclient.WithSpeed(speed)); err != nil {
		le.Printf("Could not open %s: %v\n", devPath, err)
		os.Exit(1)
	}

	blinker := New(tk)
	exit := func(code int) {
		if err := blinker.Close(); err != nil {
			le.Printf("%v\n", err)
		}
		os.Exit(code)
	}
	handleSignals(func() { exit(1) }, os.Interrupt, syscall.SIGTERM)

	if isFirmwareMode(tk) {
		le.Printf("Device is in firmware mode. Loading app...\n")
		if err := tk.LoadApp(appBinary, []byte{}); err != nil {
			le.Printf("LoadApp failed: %v", err)
			exit(1)
		}
	}

	if !isWantedApp(blinker) {
		fmt.Printf("The TKey may already be running an app, but not the expected random-app.\n" +
			"Please unplug and plug it in again.\n")
		exit(1)
	}

	err := blinker.SetPattern(pattern, patternLength)
	if err != nil {
		le.Printf("SetPattern failed: %v", err)
		exit(1)
	}

	err = blinker.Execute()
	if err != nil {
		le.Printf("Execute failed: %v", err)
		exit(1)
	}

	pattern_bytes, err := blinker.GetPattern((int)(C.pattern_step_packed_size()))
	if err != nil {
		le.Printf("GetPattern failed: %v", err)
		exit(1)
	}
	
	if bytes.Equal(pattern, pattern_bytes) {
		le.Printf("Retrieved pattern is consistent\n")
	} else {
		le.Printf("Retrieved pattern was inconsistent!\n")
		exit(1)
	}

	exit(0)
}

func handleSignals(action func(), sig ...os.Signal) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sig...)
	go func() {
		for {
			<-ch
			action()
		}
	}()
}

func isFirmwareMode(tk *tkeyclient.TillitisKey) bool {
	nameVer, err := tk.GetNameVersion()
	if err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, tkeyclient.ErrResponseStatusNotOK) {
			le.Printf("GetNameVersion failed: %s\n", err)
		}
		return false
	}
	// not caring about nameVer.Version
	return nameVer.Name0 == wantFWName0 &&
		nameVer.Name1 == wantFWName1
}

func isWantedApp(patternBlinker PatternBlinker) bool {
	nameVer, err := patternBlinker.GetAppNameVersion()
	if err != nil {
		if !errors.Is(err, io.EOF) {
			le.Printf("GetAppNameVersion: %s\n", err)
		}
		return false
	}
	// not caring about nameVer.Version
	return nameVer.Name0 == wantAppName0 &&
		nameVer.Name1 == wantAppName1
}

func GetLedData(patternString string) ([]byte, int) {
	patternString = strings.Replace(patternString, ".", ".|", -1)
	patternString = strings.Replace(patternString, "-", "-|", -1)
	patternString = strings.Replace(patternString, "|/", "/", -1)
	patternString = strings.Replace(patternString, "| ", " ", -1)

	sizeof_pattern_step_packed := (int)(C.pattern_step_packed_size())
	size := sizeof_pattern_step_packed*len(patternString)

	steps := make([]byte, size)

	for i, char := range patternString {
		duration := 0
		color := 0
		if char == '.' {
			duration = 1
			color = 7
		} else if char == '-' {
			duration = 3
			color = 7
		} else if char == '/' {
			duration = 3
		} else if char == ' ' {
			duration = 7
		} else if char == '|' {
			duration = 1
		} else {
			return nil, 0
		}

		step_padded := C.pattern_step_padded_t{color: (C.uint8_t)((uint8)(color)), duration: (C.uint8_t)((uint8)(duration))}
		C.pattern_step_pack(&step_padded, unsafe.Pointer(&steps[i * sizeof_pattern_step_packed]))
	}
	
	return steps, len(patternString)
}
