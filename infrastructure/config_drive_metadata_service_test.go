package infrastructure_test

import (
	"encoding/json"
	"errors"
	"fmt"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"

	fakeinf "github.com/cloudfoundry/bosh-agent/infrastructure/fakes"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("ConfigDriveMetadataService", describeConfigDriveMetadataService)

func describeConfigDriveMetadataService() { //nolint:funlen
	var (
		metadataService MetadataService
		resolver        *fakeinf.FakeDNSResolver
		platform        *platformfakes.FakePlatform
		logger          boshlog.Logger

		metadataServiceFileContents [][]byte
	)

	updateMetadata := func(metadataContents MetadataContentsType) {
		metadataJSON, err := json.Marshal(metadataContents)
		Expect(err).ToNot(HaveOccurred())
		metadataServiceFileContents[0] = metadataJSON

		platform.GetFilesContentsFromDiskReturns(metadataServiceFileContents, nil)
		Expect(metadataService.IsAvailable()).To(BeTrue())
	}

	updateUserdata := func(userdataContents string) {
		metadataServiceFileContents[1] = []byte(userdataContents)

		platform.GetFilesContentsFromDiskReturns(metadataServiceFileContents, nil)
		Expect(metadataService.IsAvailable()).To(BeTrue())
	}

	BeforeEach(func() {
		resolver = &fakeinf.FakeDNSResolver{}
		platform = &platformfakes.FakePlatform{}
		logger = boshlog.NewLogger(boshlog.LevelNone)
		diskPaths := []string{
			"/fake-disk-path-1",
			"/fake-disk-path-2",
		}

		metadataServiceFileContents = make([][]byte, 2)

		metadataService = NewConfigDriveMetadataService(
			resolver,
			platform,
			diskPaths,
			"fake-metadata-path",
			"fake-userdata-path",
			logger,
		)

		userdataContents := `{"server":{"name":"fake-server-name"},"registry":{"endpoint":"fake-registry-endpoint"}}`
		metadataServiceFileContents[1] = []byte(userdataContents)

		metadata := MetadataContentsType{
			PublicKeys: map[string]PublicKeyType{
				"0": PublicKeyType{
					"openssh-key": "fake-openssh-key",
				},
			},
			InstanceID: "fake-instance-id",
		}
		metadataJSON, err := json.Marshal(metadata)
		Expect(err).ToNot(HaveOccurred())
		metadataServiceFileContents[0] = metadataJSON

		updateMetadata(metadata)
	})

	Describe("GetNetworks", func() {
		It("returns the network settings", func() {
			userdataContents := `
				{
					"networks": {
						"network_1": {"type": "manual", "ip": "1.2.3.4", "netmask": "2.3.4.5", "gateway": "3.4.5.6", "default": ["dns"], "dns": ["8.8.8.8"], "mac": "fake-mac-address-1"},
						"network_2": {"type": "dynamic", "default": ["dns"], "dns": ["8.8.8.8"], "mac": "fake-mac-address-2"}
					}
				}`
			updateUserdata(userdataContents)

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
			userdataContents := `{}`
			updateUserdata(userdataContents)

			networks, err := metadataService.GetNetworks()
			Expect(err).ToNot(HaveOccurred())
			Expect(networks).To(BeNil())
		})
	})

	Describe("IsAvailable", func() {
		It("return true when it can load successfully", func() {
			Expect(metadataService.IsAvailable()).To(BeTrue())
		})

		It("returns an error if it fails to read meta-data.json from disk", func() {
			platform.GetFilesContentsFromDiskReturns([][]byte{[]byte{}, []byte{}}, errors.New("fake-read-disk-error"))
			Expect(metadataService.IsAvailable()).To(BeFalse())
		})

		It("tries to load meta-data.json from potential disk locations", func() {
			platform.GetFilesContentsFromDiskReturns([][]byte{[]byte{}, []byte{}}, errors.New("fake-read-disk-error"))
			Expect(metadataService.IsAvailable()).To(BeFalse())

			diskpath, _ := platform.GetFilesContentsFromDiskArgsForCall(1)
			Expect(diskpath).To(Equal("/fake-disk-path-1"))
			diskpath, _ = platform.GetFilesContentsFromDiskArgsForCall(2)
			Expect(diskpath).To(Equal("/fake-disk-path-2"))
		})

		It("returns an error if it fails to parse meta-data.json contents", func() {
			platform.GetFilesContentsFromDiskReturns([][]byte{[]byte("broken"), []byte{}}, nil)
			Expect(metadataService.IsAvailable()).To(BeFalse())
		})

		It("returns an error if it fails to parse user_data contents", func() {
			platform.GetFilesContentsFromDiskReturns([][]byte{[]byte{}, []byte("broken")}, nil)
			Expect(metadataService.IsAvailable()).To(BeFalse())
		})

		Context("when disk paths are not given", func() {
			It("returns false", func() {
				metadataService = NewConfigDriveMetadataService(
					resolver,
					platform,
					[]string{},
					"fake-metadata-path",
					"fake-userdata-path",
					logger,
				)
				Expect(metadataService.IsAvailable()).To(BeFalse())
			})
		})
	})

	Describe("GetPublicKey", func() {
		It("returns public key", func() {
			value, err := metadataService.GetPublicKey()
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal("fake-openssh-key"))
		})

		It("returns an error if it fails to get ssh key", func() {
			updateMetadata(MetadataContentsType{})

			value, err := metadataService.GetPublicKey()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to load openssh-key from config drive metadata service"))

			Expect(value).To(Equal(""))
		})
	})

	Describe("GetInstanceID", func() {
		It("returns instance id", func() {
			value, err := metadataService.GetInstanceID()
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal("fake-instance-id"))
		})

		It("returns an error if it fails to get instance id", func() {
			updateMetadata(MetadataContentsType{})

			value, err := metadataService.GetInstanceID()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to load instance-id from config drive metadata service"))

			Expect(value).To(Equal(""))
		})
	})

	Describe("GetServerName", func() {
		It("returns server name", func() {
			value, err := metadataService.GetServerName()
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal("fake-server-name"))
		})

		It("returns an error if it fails to get server name", func() {
			updateUserdata("{}")

			value, err := metadataService.GetServerName()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to load server name from config drive metadata service"))

			Expect(value).To(Equal(""))
		})
	})

	Describe("GetRegistryEndpoint", func() {
		It("returns an error if it fails to get registry endpoint", func() {
			updateUserdata("{}")

			value, err := metadataService.GetRegistryEndpoint()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to load registry endpoint from config drive metadata service"))

			Expect(value).To(Equal(""))
		})

		Context("when user_data does not contain a dns server", func() {
			It("returns registry endpoint", func() {
				value, err := metadataService.GetRegistryEndpoint()
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(Equal("fake-registry-endpoint"))
			})
		})

		Context("when user_data contains a dns server", func() {
			BeforeEach(func() {
				userdataContents := fmt.Sprintf(
					`{"server":{"name":"%s"},"registry":{"endpoint":"%s"},"dns":{"nameserver":["%s"]}}`,
					"fake-server-name",
					"http://fake-registry.com",
					"fake-dns-server-ip",
				)
				updateUserdata(userdataContents)
			})

			Context("when registry endpoint is successfully resolved", func() {
				BeforeEach(func() {
					resolver.RegisterRecord(fakeinf.FakeDNSRecord{
						DNSServers: []string{"fake-dns-server-ip"},
						Host:       "http://fake-registry.com",
						IP:         "http://fake-registry-ip",
					})
				})

				It("returns the successfully resolved registry endpoint", func() {
					endpoint, err := metadataService.GetRegistryEndpoint()
					Expect(err).ToNot(HaveOccurred())
					Expect(endpoint).To(Equal("http://fake-registry-ip"))
				})
			})

			Context("when registry endpoint is not successfully resolved", func() {
				BeforeEach(func() {
					resolver.LookupHostErr = errors.New("fake-lookup-host-err")
				})

				It("returns error because it failed to resolve registry endpoint", func() {
					endpoint, err := metadataService.GetRegistryEndpoint()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-lookup-host-err"))
					Expect(endpoint).To(BeEmpty())
				})
			})
		})
	})

	Describe("GetSettings", func() {
		It("returns an error if it fails to get settings", func() {
			updateUserdata("{}")

			_, err := metadataService.GetSettings()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Metadata does not provide settings"))
		})

		Context("when user_data contains settings", func() {
			BeforeEach(func() {
				userdataContents := fmt.Sprintf(
					`
{
	"server":{"name":"%s"},
	"registry":{"endpoint":"%s"},
	"dns":{"nameserver":["%s"]},
	"agent_id":"%s",
	"mbus": "%s"
}`,
					"fake-server-name",
					"http://fake-registry.com",
					"fake-dns-server-ip",
					"Agent-Foo",
					"Agent-Mbus",
				)

				updateUserdata(userdataContents)
			})

			It("returns the settings", func() {
				settings, err := metadataService.GetSettings()
				Expect(err).ToNot(HaveOccurred())
				Expect(settings.AgentID).To(Equal("Agent-Foo"))
			})
		})
	})
}
