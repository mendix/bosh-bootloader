package commands_test

import (
	"bytes"

	"github.com/pivotal-cf-experimental/bosh-bootloader/commands"
	"github.com/pivotal-cf-experimental/bosh-bootloader/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Version", func() {
	var (
		version commands.Version
		stdout  *bytes.Buffer
	)

	BeforeEach(func() {
		stdout = bytes.NewBuffer([]byte{})
		version = commands.NewVersion(stdout)
	})

	Describe("Execute", func() {
		It("prints out the version information", func() {
			_, err := version.Execute([]string{}, storage.State{})
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout.String()).To(Equal("bbl 0.0.1\n"))
		})

		It("returns the given state without modification", func() {
			state, err := version.Execute([]string{}, storage.State{
				Version: 3,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(state).To(Equal(storage.State{
				Version: 3,
			}))
		})
	})
})
