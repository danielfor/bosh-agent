package infrastructure_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("FileMetadataService", func() {
	var (
		fs              *fakesys.FakeFileSystem
		metadataService MetadataService
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		logger := boshlog.NewLogger(boshlog.LevelNone)
		metadataService = NewFileMetadataService(
			"fake-metadata-file-path",
			"fake-userdata-file-path",
			"fake-settings-file-path",
			fs,
			logger,
		)
	})

	Describe("GetInstanceID", func() {
		Context("when metadata service file exists", func() {
			BeforeEach(func() {
				err := fs.WriteFileString("fake-metadata-file-path", `{"instance-id":"fake-instance-id"}`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns instance id", func() {
				instanceID, err := metadataService.GetInstanceID()
				Expect(err).NotTo(HaveOccurred())
				Expect(instanceID).To(Equal("fake-instance-id"))
			})
		})

		Context("when metadata service file does not exist", func() {
			It("returns an error", func() {
				_, err := metadataService.GetInstanceID()
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when metadata service file has invalid format", func() {
			BeforeEach(func() {
				err := fs.WriteFileString("fake-metadata-file-path", "bad-json")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				_, err := metadataService.GetInstanceID()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("GetServerName", func() {
		Context("when userdata file exists", func() {
			BeforeEach(func() {
				err := fs.WriteFileString("fake-userdata-file-path", `{"server":{"name":"fake-server-name"}}`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns server name", func() {
				serverName, err := metadataService.GetServerName()
				Expect(err).NotTo(HaveOccurred())
				Expect(serverName).To(Equal("fake-server-name"))
			})
		})

		Context("when userdata file does not exist", func() {
			It("returns an error", func() {
				serverName, err := metadataService.GetServerName()
				Expect(err).To(HaveOccurred())
				Expect(serverName).To(BeEmpty())
			})
		})
	})

	Describe("GetNetworks", func() {
		It("returns the network settings", func() {
			userDataContents := `
				{
					"networks": {
						"network_1": {"type": "manual", "ip": "1.2.3.4", "netmask": "2.3.4.5", "gateway": "3.4.5.6", "default": ["dns"], "dns": ["8.8.8.8"], "mac": "fake-mac-address-1"},
						"network_2": {"type": "dynamic", "default": ["dns"], "dns": ["8.8.8.8"], "mac": "fake-mac-address-2"}
					}
				}`
			err := fs.WriteFileString("fake-userdata-file-path", userDataContents)
			Expect(err).NotTo(HaveOccurred())

			networks, err := metadataService.GetNetworks()
			Expect(err).ToNot(HaveOccurred())
			Expect(networks).To(Equal(boshsettings.Networks{
				"network_1": boshsettings.Network{
					Type:    "manual",
					IP:      "1.2.3.4",
					Netmask: "2.3.4.5",
					Gateway: "3.4.5.6",
					Default: []string{"dns"},
					DNS:     []string{"8.8.8.8"},
					Mac:     "fake-mac-address-1",
				},
				"network_2": boshsettings.Network{
					Type:    "dynamic",
					Default: []string{"dns"},
					DNS:     []string{"8.8.8.8"},
					Mac:     "fake-mac-address-2",
				},
			}))
		})

		It("returns a nil Networks if the settings are missing (from an old CPI version)", func() {
			userDataContents := `{}`
			err := fs.WriteFileString("fake-userdata-file-path", userDataContents)
			Expect(err).NotTo(HaveOccurred())

			networks, err := metadataService.GetNetworks()
			Expect(err).ToNot(HaveOccurred())
			Expect(networks).To(BeNil())
		})

		It("raises an error if we can't read the file", func() {
			networks, err := metadataService.GetNetworks()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Reading user data:"))
			be, ok := err.(bosherr.ComplexError)
			Expect(ok).To(BeTrue())
			be, ok = be.Cause.(bosherr.ComplexError)
			Expect(ok).To(BeTrue())
			pe, ok := be.Cause.(*os.PathError)
			Expect(ok).To(BeTrue())
			Expect(os.IsNotExist(pe)).To(BeTrue())
			Expect(networks).To(BeNil())
		})
	})

	Describe("GetRegistryEndpoint", func() {
		Context("when metadata service file exists", func() {
			BeforeEach(func() {
				err := fs.WriteFileString(
					"fake-userdata-file-path",
					`{"registry":{"endpoint":"fake-registry-endpoint"}}`,
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns registry endpoint", func() {
				registryEndpoint, err := metadataService.GetRegistryEndpoint()
				Expect(err).NotTo(HaveOccurred())
				Expect(registryEndpoint).To(Equal("fake-registry-endpoint"))
			})
		})

		Context("when metadata service file does not exist", func() {
			It("returns registry endpoint pointing to a settings file", func() {
				registryEndpoint, err := metadataService.GetRegistryEndpoint()
				Expect(err).NotTo(HaveOccurred())
				Expect(registryEndpoint).To(Equal("fake-settings-file-path"))
			})
		})
	})

	Describe("GetSettings", func() {
		Context("when metadata service file exists", func() {
			BeforeEach(func() {
				userDataContents := `
				{
					"registry":{"endpoint":"fake-registry-endpoint"},
					"agent_id":"Agent-Foo",
					"mbus": "Agent-Mbus"
				}`

				err := fs.WriteFileString("fake-userdata-file-path", userDataContents)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns settings", func() {
				settings, err := metadataService.GetSettings()
				Expect(err).NotTo(HaveOccurred())
				Expect(settings.AgentID).To(Equal("Agent-Foo"))
			})

			Context("when metadata settings does NOT contain agentID", func() {
				BeforeEach(func() {
					userDataContents := `
					{
						"registry":{"endpoint":"fake-registry-endpoint"},
						"mbus": "Agent-Mbus"
					}`

					err := fs.WriteFileString("fake-userdata-file-path", userDataContents)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns error", func() {
					_, err := metadataService.GetSettings()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Metadata does not provide settings"))
				})
			})
		})

		Context("when metadata service file does not exist", func() {
			BeforeEach(func() {
				err := fs.RemoveAll("fake-settings-file-path")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns error", func() {
				_, err := metadataService.GetSettings()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Reading user data: Not found: open fake-userdata-file-path"))
			})
		})

		Context("when we have incorrect metadata in file", func() {
			BeforeEach(func() {
				userDataContents := `
					{
						"INCORRECT JSON": ,
						"registry":{"endpoint":"fake-registry-endpoint"},
						"settings":{
							"mbus": "Agent-Mbus"
					}`

				err := fs.WriteFileString("fake-userdata-file-path", userDataContents)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns error", func() {
				_, err := metadataService.GetSettings()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Unmarshalling user data: invalid character ',' looking for beginning of value"))
			})
		})
	})

	Describe("IsAvailable", func() {
		Context("when file does not exist", func() {
			It("returns false", func() {
				Expect(metadataService.IsAvailable()).To(BeFalse())
			})
		})

		Context("when file exists", func() {
			BeforeEach(func() {
				err := fs.WriteFileString("fake-settings-file-path", ``)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns true", func() {
				Expect(metadataService.IsAvailable()).To(BeTrue())
			})
		})
	})
})
