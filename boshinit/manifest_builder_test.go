package boshinit_test

import (
	"io/ioutil"

	"github.com/cloudfoundry-incubator/candiedyaml"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/bosh-bootloader/boshinit"
	"github.com/pivotal-cf-experimental/bosh-bootloader/fakes"
	. "github.com/pivotal-cf-experimental/gomegamatchers"
)

var _ = Describe("ManifestBuilder", func() {
	var (
		logger             *fakes.Logger
		manifestBuilder    boshinit.ManifestBuilder
		manifestProperties boshinit.ManifestProperties
	)

	BeforeEach(func() {
		logger = &fakes.Logger{}
		manifestBuilder = boshinit.NewManifestBuilder(logger)
		manifestProperties = boshinit.ManifestProperties{
			SubnetID: "subnet-12345",
		}
	})

	Describe("Build", func() {
		It("builds the bosh-init manifest", func() {
			manifest := manifestBuilder.Build(manifestProperties)

			Expect(manifest.Name).To(Equal("bosh"))
			Expect(manifest.Releases[0].Name).To(Equal("bosh"))
			Expect(manifest.ResourcePools[0].Name).To(Equal("vms"))
			Expect(manifest.DiskPools[0].Name).To(Equal("disks"))
			Expect(manifest.Networks[0].Subnets[0].CloudProperties.Subnet).To(Equal("subnet-12345"))
			Expect(manifest.Jobs[0].Name).To(Equal("bosh"))
			Expect(manifest.CloudProvider.Template.Name).To(Equal("aws_cpi"))

		})

		It("logs that the bosh-init manifest is being generated", func() {
			manifestBuilder.Build(manifestProperties)
			Expect(logger.StepCall.Receives.Message).To(Equal("generating bosh-init manifest"))
		})
	})

	Describe("template marshaling", func() {
		It("can be marshaled to YML", func() {
			manifest := manifestBuilder.Build(manifestProperties)

			buf, err := ioutil.ReadFile("fixtures/manifest.yml")
			Expect(err).NotTo(HaveOccurred())

			output, err := candiedyaml.Marshal(manifest)
			Expect(err).NotTo(HaveOccurred())

			Expect(output).To(MatchYAML(string(buf)))
		})
	})
})