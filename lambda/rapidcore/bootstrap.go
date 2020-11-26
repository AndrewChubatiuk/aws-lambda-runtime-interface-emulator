// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import (
	"fmt"
	"os"

	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/logging"
	"go.amzn.com/lambda/rapid"

	log "github.com/sirupsen/logrus"
)

type LogFormatter func(error) string
type BootstrapError func() (fatalerror.ErrorType, LogFormatter)

// Bootstrap represents a list of executable bootstrap
// candidates in order of priority and exec metadata
type Bootstrap struct {
	orderedLookupPaths []string
	validCmd           []string
	workingDir         string
	cmdCandidates      [][]string
	extraFiles         []*os.File
	bootstrapError     BootstrapError
}

// NewBootstrap returns an instance of bootstrap defined by given params
func NewBootstrap(cmdCandidates [][]string, currentWorkingDir string) *Bootstrap {
	var orderedLookupBootstrapPaths []string
	for _, args := range cmdCandidates {
		// Empty args is an error, but we want to detect it later (in Cmd() call) when we are able to report a descriptive error
		if len(args) != 0 {
			orderedLookupBootstrapPaths = append(orderedLookupBootstrapPaths, args[0])
		}
	}
	return &Bootstrap{
		orderedLookupPaths: orderedLookupBootstrapPaths,
		workingDir:         currentWorkingDir,
		cmdCandidates:      cmdCandidates,
	}
}

// locateBootstrap sets the first occurrence of an
// actual bootstrap, given a list of possible files
func (b *Bootstrap) locateBootstrap() error {
	for i, bootstrapCandidate := range b.orderedLookupPaths {
		if file, err := os.Stat(bootstrapCandidate); !os.IsNotExist(err) && !file.IsDir() {
			b.validCmd = b.cmdCandidates[i]
			return nil
		}
	}
	log.WithField("bootstrapPathsChecked", b.orderedLookupPaths).Warn("Couldn't find valid bootstrap(s)")
	return fmt.Errorf("Couldn't find valid bootstrap(s): %s", b.orderedLookupPaths)
}

// Cmd returns the args of bootstrap, where args[0]
// is the path to executable
func (b *Bootstrap) Cmd() ([]string, error) {
	if err := b.locateBootstrap(); err != nil {
		return []string{}, err
	}

	log.Debug("Located runtime bootstrap", b.validCmd[0])
	return b.validCmd, nil
}

// Env returns the environment variables available to
// the bootstrap process
func (b *Bootstrap) Env(e rapid.EnvironmentVariables) []string {
	return e.RuntimeExecEnv()
}

// Cwd returns the working directory of the bootstrap process
func (b *Bootstrap) Cwd() string {
	return b.workingDir
}

// SetExtraFiles sets the extra file descriptors apart from 1 & 2 to be passed to runtime
func (b *Bootstrap) SetExtraFiles(extraFiles []*os.File) {
	b.extraFiles = extraFiles
}

// ExtraFiles returns the extra file descriptors apart from 1 & 2 to be passed to runtime
func (b *Bootstrap) ExtraFiles() []*os.File {
	return b.extraFiles
}

// CachedFatalError returns a bootstrap error that occurred during startup and before init
// so that it can be reported back to the customer in a later phase
func (b *Bootstrap) CachedFatalError(err error) (fatalerror.ErrorType, string, bool) {
	if b.bootstrapError == nil {
		return fatalerror.ErrorType(""), "", false
	}

	fatalError, logFunc := b.bootstrapError()

	return fatalError, logFunc(err), true
}

// SetCachedFatalError sets a cached fatal error that occurred during startup and before init
// so that it can be reported back to the customer in a later phase
func (b *Bootstrap) SetCachedFatalError(bootstrapErrFn BootstrapError) {
	b.bootstrapError = bootstrapErrFn
}

// BootstrapErrInvalidOCITaskConfig represents an error while parsing OCI task config
func BootstrapErrInvalidOCITaskConfig(err error) BootstrapError {
	return func() (fatalerror.ErrorType, LogFormatter) {
		return fatalerror.InvalidTaskConfig, logging.SupernovaInvalidTaskConfigRepr(err)
	}
}

// BootstrapErrInvalidOCIEntrypoint represents an invalid OCI entrypoint error
func BootstrapErrInvalidOCIEntrypoint(entrypoint []string, cmd []string, workingdir string) BootstrapError {
	return func() (fatalerror.ErrorType, LogFormatter) {
		return fatalerror.InvalidEntrypoint, logging.SupernovaLaunchErrorRepr(entrypoint, cmd, workingdir)
	}
}
