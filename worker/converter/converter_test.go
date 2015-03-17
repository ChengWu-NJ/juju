// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package converter_test

import (
	"os"
	"os/signal"
	"runtime"
	stdtesting "testing"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/testing"
	"github.com/juju/juju/worker"
	"github.com/juju/juju/worker/converter"
)

func TestPackage(t *stdtesting.T) {
	gc.TestingT(t)
}

var _ = gc.Suite(&ConverterSuite{})

type ConverterSuite struct {
	testing.BaseSuite
	// c is a channel that will wait for the termination
	// signal, to prevent signals terminating the process.
	c chan os.Signal
}

func (s *ConverterSuite) SetUpTest(c *gc.C) {
	s.BaseSuite.SetUpTest(c)
	s.c = make(chan os.Signal, 1)
	signal.Notify(s.c, converter.TerminationSignal)
}

func (s *ConverterSuite) TearDownTest(c *gc.C) {
	close(s.c)
	signal.Stop(s.c)
	s.BaseSuite.TearDownTest(c)
}

func (s *ConverterSuite) TestStartStop(c *gc.C) {
	w := converter.NewWorker()
	w.Kill()
	err := w.Wait()
	c.Assert(err, jc.ErrorIsNil)
}

func (s *ConverterSuite) TestSignal(c *gc.C) {
	//TODO(bogdanteleaga): Inspect this further on windows
	if runtime.GOOS == "windows" {
		c.Skip("bug 1403084: sending this signal is not supported on windows")
	}
	w := converter.NewWorker()
	proc, err := os.FindProcess(os.Getpid())
	c.Assert(err, jc.ErrorIsNil)
	defer proc.Release()
	err = proc.Signal(converter.TerminationSignal)
	c.Assert(err, jc.ErrorIsNil)
	err = w.Wait()
	c.Assert(err, gc.Equals, worker.ErrTerminateAgent)
}
