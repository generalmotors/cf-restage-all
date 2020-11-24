package main_test

import (
	"code.cloudfoundry.org/cli/cf/util/testhelpers/pluginbuilder"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRestageAllCLIPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	pluginbuilder.BuildTestBinary("", "restage_all")
	RunSpecs(t, "TestRestageAllCLIPlugin Suite")
}

var _ = BeforeEach(func() {
	SetDefaultEventuallyTimeout(3 * time.Second)
})
