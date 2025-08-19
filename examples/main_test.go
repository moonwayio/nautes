package main

import (
	"os"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/suite"
)

type MainTestSuite struct {
	suite.Suite
}

func (s *MainTestSuite) TestMain() {
	signalChan := make(chan os.Signal, 1)

	wg := sync.WaitGroup{}
	wg.Add(1)
	var err error
	go func() {
		defer wg.Done()
		err = Run(signalChan)
	}()

	signalChan <- syscall.SIGINT

	wg.Wait()

	s.Require().NoError(err)
}

func TestMainTestSuite(t *testing.T) {
	suite.Run(t, new(MainTestSuite))
}
