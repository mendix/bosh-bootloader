package commands_test

import (
	"errors"
	"fmt"

	"github.com/pivotal-cf-experimental/bosh-bootloader/aws/cloudformation"
	"github.com/pivotal-cf-experimental/bosh-bootloader/aws/iam"
	"github.com/pivotal-cf-experimental/bosh-bootloader/bosh"
	"github.com/pivotal-cf-experimental/bosh-bootloader/commands"
	"github.com/pivotal-cf-experimental/bosh-bootloader/fakes"
	"github.com/pivotal-cf-experimental/bosh-bootloader/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Create LBs", func() {
	Describe("Execute", func() {
		var (
			command                   commands.CreateLBs
			certificateManager        *fakes.CertificateManager
			infrastructureManager     *fakes.InfrastructureManager
			boshClient                *fakes.BOSHClient
			boshClientProvider        *fakes.BOSHClientProvider
			availabilityZoneRetriever *fakes.AvailabilityZoneRetriever
			boshCloudConfigurator     *fakes.BoshCloudConfigurator
			awsCredentialValidator    *fakes.AWSCredentialValidator
			logger                    *fakes.Logger
			cloudConfigManager        *fakes.CloudConfigManager
			certificateValidator      *fakes.CertificateValidator
			incomingState             storage.State
		)

		BeforeEach(func() {
			certificateManager = &fakes.CertificateManager{}
			infrastructureManager = &fakes.InfrastructureManager{}
			availabilityZoneRetriever = &fakes.AvailabilityZoneRetriever{}
			boshCloudConfigurator = &fakes.BoshCloudConfigurator{}
			boshClient = &fakes.BOSHClient{}
			boshClientProvider = &fakes.BOSHClientProvider{}
			awsCredentialValidator = &fakes.AWSCredentialValidator{}
			logger = &fakes.Logger{}
			cloudConfigManager = &fakes.CloudConfigManager{}
			certificateValidator = &fakes.CertificateValidator{}

			boshClientProvider.ClientCall.Returns.Client = boshClient

			infrastructureManager.ExistsCall.Returns.Exists = true

			incomingState = storage.State{
				Stack: storage.Stack{
					Name: "some-stack",
				},
				AWS: storage.AWS{
					AccessKeyID:     "some-access-key-id",
					SecretAccessKey: "some-secret-access-key",
					Region:          "some-region",
				},
				KeyPair: storage.KeyPair{
					Name: "some-key-pair",
				},
				BOSH: storage.BOSH{
					DirectorAddress:  "some-director-address",
					DirectorUsername: "some-director-username",
					DirectorPassword: "some-director-password",
				},
			}

			command = commands.NewCreateLBs(logger, awsCredentialValidator, certificateManager, infrastructureManager,
				availabilityZoneRetriever, boshClientProvider, boshCloudConfigurator, cloudConfigManager, certificateValidator)
		})

		It("returns an error if aws credential validator fails", func() {
			awsCredentialValidator.ValidateCall.Returns.Error = errors.New("failed to validate aws credentials")
			_, err := command.Execute([]string{}, storage.State{})
			Expect(err).To(MatchError("failed to validate aws credentials"))
		})

		It("uploads a cert and key", func() {
			_, err := command.Execute([]string{
				"--type", "concourse",
				"--cert", "temp/some-cert.crt",
				"--key", "temp/some-key.key",
			}, storage.State{})
			Expect(err).NotTo(HaveOccurred())

			Expect(certificateManager.CreateCall.Receives.Certificate).To(Equal("temp/some-cert.crt"))
			Expect(certificateManager.CreateCall.Receives.PrivateKey).To(Equal("temp/some-key.key"))
			Expect(logger.StepCall.Messages).To(ContainElement("uploading certificate"))
		})

		It("uploads a cert and key with chain", func() {
			_, err := command.Execute([]string{
				"--type", "concourse",
				"--cert", "temp/some-cert.crt",
				"--key", "temp/some-key.key",
				"--chain", "temp/some-chain.crt",
			}, storage.State{})
			Expect(err).NotTo(HaveOccurred())

			Expect(certificateManager.CreateCall.Receives.Chain).To(Equal("temp/some-chain.crt"))

			Expect(certificateValidator.ValidateCall.Receives.Command).To(Equal("unsupported-create-lbs"))
			Expect(certificateValidator.ValidateCall.Receives.CertificatePath).To(Equal("temp/some-cert.crt"))
			Expect(certificateValidator.ValidateCall.Receives.KeyPath).To(Equal("temp/some-key.key"))
			Expect(certificateValidator.ValidateCall.Receives.ChainPath).To(Equal("temp/some-chain.crt"))
		})

		It("creates a load balancer in cloudformation with certificate", func() {
			availabilityZoneRetriever.RetrieveCall.Returns.AZs = []string{"a", "b", "c"}
			certificateManager.CreateCall.Returns.CertificateName = "some-certificate-name"
			certificateManager.DescribeCall.Returns.Certificate = iam.Certificate{
				ARN: "some-certificate-arn",
			}
			_, err := command.Execute([]string{
				"--type", "concourse",
				"--cert", "temp/some-cert.crt",
				"--key", "temp/some-key.key",
			}, incomingState)
			Expect(err).NotTo(HaveOccurred())

			Expect(availabilityZoneRetriever.RetrieveCall.Receives.Region).To(Equal("some-region"))

			Expect(certificateManager.DescribeCall.Receives.CertificateName).To(Equal("some-certificate-name"))

			Expect(infrastructureManager.UpdateCall.Receives.KeyPairName).To(Equal("some-key-pair"))
			Expect(infrastructureManager.UpdateCall.Receives.NumberOfAvailabilityZones).To(Equal(3))
			Expect(infrastructureManager.UpdateCall.Receives.StackName).To(Equal("some-stack"))
			Expect(infrastructureManager.UpdateCall.Receives.LBType).To(Equal("concourse"))
			Expect(infrastructureManager.UpdateCall.Receives.LBCertificateARN).To(Equal("some-certificate-arn"))
		})

		It("updates the cloud config with lb type", func() {
			infrastructureManager.UpdateCall.Returns.Stack = cloudformation.Stack{
				Name: "some-stack",
			}
			availabilityZoneRetriever.RetrieveCall.Returns.AZs = []string{"a", "b", "c"}
			boshCloudConfigurator.ConfigureCall.Returns.CloudConfigInput = bosh.CloudConfigInput{
				AZs: []string{"a", "b", "c"},
			}

			_, err := command.Execute([]string{
				"--type", "concourse",
				"--cert", "temp/some-cert.crt",
				"--key", "temp/some-key.key",
			}, incomingState)
			Expect(err).NotTo(HaveOccurred())

			Expect(boshCloudConfigurator.ConfigureCall.Receives.Stack).To(Equal(cloudformation.Stack{
				Name: "some-stack",
			}))
			Expect(boshCloudConfigurator.ConfigureCall.Receives.AZs).To(Equal([]string{"a", "b", "c"}))

			Expect(cloudConfigManager.UpdateCall.Receives.CloudConfigInput).To(Equal(bosh.CloudConfigInput{
				AZs: []string{"a", "b", "c"},
			}))
		})

		Context("when --skip-if-exists is provided", func() {
			It("no-ops when lb exists", func() {
				incomingState.Stack.LBType = "cf"
				_, err := command.Execute([]string{
					"--type", "concourse",
					"--cert", "temp/some-cert.crt",
					"--key", "temp/some-key.key",
					"--skip-if-exists",
				}, incomingState)
				Expect(err).NotTo(HaveOccurred())

				Expect(infrastructureManager.UpdateCall.CallCount).To(Equal(0))
				Expect(certificateManager.CreateCall.CallCount).To(Equal(0))

				Expect(logger.PrintlnCall.Receives.Message).To(Equal(`lb type "cf" exists, skipping...`))
			})

			DescribeTable("creates the lb if the lb does not exist",
				func(currentLBType string) {
					incomingState.Stack.LBType = currentLBType
					_, err := command.Execute([]string{
						"--type", "concourse",
						"--cert", "temp/some-cert.crt",
						"--key", "temp/some-key.key",
						"--skip-if-exists",
					}, incomingState)
					Expect(err).NotTo(HaveOccurred())

					Expect(infrastructureManager.UpdateCall.CallCount).To(Equal(1))
					Expect(certificateManager.CreateCall.CallCount).To(Equal(1))
				},
				Entry("when the current lb-type is 'none'", "none"),
				Entry("when the current lb-type is ''", ""),
			)
		})

		Context("invalid lb type", func() {
			It("returns an error", func() {
				_, err := command.Execute([]string{
					"--type", "some-invalid-lb",
					"--cert", "temp/some-cert.crt",
					"--key", "temp/some-key.key",
				}, storage.State{})
				Expect(err).To(MatchError("\"some-invalid-lb\" is not a valid lb type, valid lb types are: concourse and cf"))
			})
		})

		Context("fast fail if the stack or BOSH director does not exist", func() {
			It("returns an error when the stack does not exist", func() {
				infrastructureManager.ExistsCall.Returns.Exists = false

				_, err := command.Execute([]string{
					"--type", "concourse",
					"--cert", "temp/some-cert.crt",
					"--key", "temp/some-key.key",
				}, incomingState)

				Expect(infrastructureManager.ExistsCall.Receives.StackName).To(Equal("some-stack"))

				Expect(err).To(MatchError(commands.BBLNotFound))
			})

			It("returns an error when the BOSH director does not exist", func() {
				boshClient.InfoCall.Returns.Error = errors.New("director not found")
				infrastructureManager.ExistsCall.Returns.Exists = true

				_, err := command.Execute([]string{
					"--type", "concourse",
					"--cert", "temp/some-cert.crt",
					"--key", "temp/some-key.key",
				}, incomingState)

				Expect(boshClientProvider.ClientCall.Receives.DirectorAddress).To(Equal("some-director-address"))
				Expect(boshClientProvider.ClientCall.Receives.DirectorUsername).To(Equal("some-director-username"))
				Expect(boshClientProvider.ClientCall.Receives.DirectorPassword).To(Equal("some-director-password"))

				Expect(boshClient.InfoCall.CallCount).To(Equal(1))

				Expect(err).To(MatchError(commands.BBLNotFound))
			})
		})

		Context("state manipulation", func() {
			It("returns a state with new certificate name and lb type", func() {
				certificateManager.CreateCall.Returns.CertificateName = "some-certificate-name"
				state, err := command.Execute([]string{
					"--type", "concourse",
					"--cert", "temp/some-cert.crt",
					"--key", "temp/some-key.key",
				}, storage.State{})
				Expect(err).NotTo(HaveOccurred())

				Expect(state.Stack.CertificateName).To(Equal("some-certificate-name"))
				Expect(state.Stack.LBType).To(Equal("concourse"))
			})
		})

		Context("required args", func() {
			It("returns an error when certificate validator fails for cert and key", func() {
				certificateValidator.ValidateCall.Returns.Error = errors.New("failed to validate")
				_, err := command.Execute([]string{
					"--type", "concourse",
					"--cert", "/path/to/cert",
					"--key", "/path/to/key",
				}, storage.State{})

				Expect(err).To(MatchError("failed to validate"))
				Expect(certificateValidator.ValidateCall.Receives.Command).To(Equal("unsupported-create-lbs"))
				Expect(certificateValidator.ValidateCall.Receives.CertificatePath).To(Equal("/path/to/cert"))
				Expect(certificateValidator.ValidateCall.Receives.KeyPath).To(Equal("/path/to/key"))
				Expect(certificateValidator.ValidateCall.Receives.ChainPath).To(Equal(""))

				Expect(certificateManager.CreateCall.CallCount).To(Equal(0))
			})
		})

		Context("failure cases", func() {
			DescribeTable("returns an error when an lb already exists",
				func(newLbType, oldLbType string) {
					_, err := command.Execute([]string{
						"--type", "concourse",
						"--cert", "/path/to/cert",
						"--key", "/path/to/key",
					}, storage.State{
						Stack: storage.Stack{
							LBType: oldLbType,
						},
					})
					Expect(err).To(MatchError(fmt.Sprintf("bbl already has a %s load balancer attached, please remove the previous load balancer before attaching a new one", oldLbType)))
				},
				Entry("when the previous lb type is concourse", "concourse", "cf"),
				Entry("when the previous lb type is cf", "cf", "concourse"),
			)

			It("returns an error when the infrastructure manager fails to check the existance of a stack", func() {
				infrastructureManager.ExistsCall.Returns.Error = errors.New("failed to check for stack")
				_, err := command.Execute([]string{
					"--type", "concourse",
					"--cert", "/path/to/cert",
					"--key", "/path/to/key",
				}, storage.State{})
				Expect(err).To(MatchError("failed to check for stack"))
			})

			Context("when an invalid command line flag is supplied", func() {
				It("returns an error", func() {
					_, err := command.Execute([]string{"--invalid-flag"}, storage.State{})
					Expect(err).To(MatchError("flag provided but not defined: -invalid-flag"))
				})
			})

			Context("when availability zone retriever fails", func() {
				It("returns an error", func() {
					availabilityZoneRetriever.RetrieveCall.Returns.Error = errors.New("failed to retrieve azs")

					_, err := command.Execute([]string{
						"--type", "concourse",
						"--cert", "/path/to/cert",
						"--key", "/path/to/key",
					}, storage.State{})
					Expect(err).To(MatchError("failed to retrieve azs"))
				})
			})

			Context("when update infrastructure manager fails", func() {
				It("returns an error", func() {
					infrastructureManager.UpdateCall.Returns.Error = errors.New("failed to update infrastructure")

					_, err := command.Execute([]string{
						"--type", "concourse",
						"--cert", "/path/to/cert",
						"--key", "/path/to/key",
					}, storage.State{})
					Expect(err).To(MatchError("failed to update infrastructure"))
				})
			})

			Context("when certificate manager fails to create a certificate", func() {
				It("returns an error", func() {
					certificateManager.CreateCall.Returns.Error = errors.New("failed to create cert")

					_, err := command.Execute([]string{
						"--type", "concourse",
						"--cert", "/path/to/cert",
						"--key", "/path/to/key",
					}, storage.State{})
					Expect(err).To(MatchError("failed to create cert"))
				})
			})

			Context("when cloud config manager update fails", func() {
				It("returns an error", func() {
					cloudConfigManager.UpdateCall.Returns.Error = errors.New("failed to update cloud config")

					_, err := command.Execute([]string{
						"--type", "concourse",
						"--cert", "/path/to/cert",
						"--key", "/path/to/key",
					}, storage.State{})
					Expect(err).To(MatchError("failed to update cloud config"))
				})
			})
		})
	})
})
