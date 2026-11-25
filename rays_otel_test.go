package rays_otel_test

import (
	"testing"

	_ "github.com/BeamFoundry/rays-otel"
	. "github.com/BeamFoundry/rays/pkg/testing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRaysOtel(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RaysOtel Suite")
}

var _ = Describe("RaysOtel", func() {
	It("registers default otel configuration on import", func() {
		Expect(GetContainer().Initialize()).NotTo(HaveOccurred())
		val, ok := GetContainer().ConfigValue("rays.otel.serviceName")
		Expect(ok).To(BeTrue())
		Expect(val).To(Equal("rays-app"))
	})
})
