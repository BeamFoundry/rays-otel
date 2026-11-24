package autoconfig

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAutoconfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Internal/Autoconfig Suite")
}
