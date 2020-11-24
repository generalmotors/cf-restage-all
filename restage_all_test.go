package main_test

import (
	"code.cloudfoundry.org/cli/cf/util/testhelpers/rpcserver"
	"code.cloudfoundry.org/cli/cf/util/testhelpers/rpcserver/rpcserverfakes"
	pluginmodels "code.cloudfoundry.org/cli/plugin/models"
	"errors"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

const validPluginPath = "./restage_all.exe"

var _ = Describe("RestageAllCLIPlugin", func() {

	var (
		rpcHandlers *rpcserverfakes.FakeHandlers
		ts          *rpcserver.TestServer
		err         error
	)

	BeforeEach(func() {
		rpcHandlers = new(rpcserverfakes.FakeHandlers)
		ts, err = rpcserver.NewTestRPCServer(rpcHandlers)
		Expect(err).NotTo(HaveOccurred())

		rpcHandlers.CallCoreCommandStub = func(_ []string, retVal *bool) error {
			*retVal = true
			return nil
		}

		rpcHandlers.GetOutputAndResetStub = func(_ bool, retVal *[]string) error {
			*retVal = []string{"{}"}
			return nil
		}
	})

	JustBeforeEach(func() {
		err = ts.Start()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ts.Stop()
	})

	Describe("restage-all", func() {
		Context("Running the command", func() {
			Context("curling endpoints", func() {
				BeforeEach(func() {
					rpcHandlers.ApiEndpointStub = func(_ string, retVal *string) error {
						*retVal = "api.example.com"
						return nil
					}

					rpcHandlers.IsMinCliVersionStub = func(version string, retVal *bool) error {
						*retVal = true
						return nil
					}
				})

				Context("getting a list of apps", func() {
					BeforeEach(func() {
						count := 0
						rpcHandlers.GetOutputAndResetStub = func(_ bool, retVal *[]string) error {
							switch count {
							case 0:
								fallthrough
							case 1:
								{
									// /v3/apps/{guid}/droplets/current
									*retVal = []string{`{"created_at": "2020-03-28T23:39:34Z", "links": { "package": { "href": "pkg" } } }`}
								}
							case 2:
								{
									// /v3/builds
									*retVal = []string{`{"guid": "1234"}`}
								}
							case 3:
								{
									// /v3/builds/{guid}
									*retVal = []string{`{"state": "STAGED", "guid": "1234"}`}
								}
							case 4:
								{
									// /v3/builds/{guid}
									*retVal = []string{`{"droplet": {"guid": "abcd"}}`}
								}
							case 5:
								{
									// /v3/apps/{guid}/relationships/current_droplet
									*retVal = []string{`{"data": {"guid": "abcd"}}`}
								}
							case 6:
								{
									// /v3/apps/{guid}/actions/restart
									*retVal = []string{`{"State": "STARTING"}`}
								}
							case 7:
								{
									// /v3/apps/{guid}
									*retVal = []string{`{"State": "STARTED"}`}
								}
							}
							count++
							return nil
						}
						rpcHandlers.GetAppsStub = func(_ string, retVal *[]pluginmodels.GetAppsModel) error {
							*retVal = sampleApps()
							return nil
						}
						rpcHandlers.GetAppStub = func(_ string, retVal *pluginmodels.GetAppModel) error {
							*retVal = pluginmodels.GetAppModel{
								State: "STARTED",
							}
							return nil
						}
					})

					When("there is an issue getting apps", func() {
						BeforeEach(func() {
							rpcHandlers.GetAppsStub = func(_ string, retVal *[]pluginmodels.GetAppsModel) error {
								return errors.New("some horrible error occurred")
							}
						})

						It("raises an error", func() {
							args := []string{ts.Port(), "restage-all"}
							session, err := gexec.Start(exec.Command(validPluginPath, args...), GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())
							session.Wait()
							Expect(session).To(gbytes.Say("some horrible error occurred"))
							Expect(session.ExitCode()).To(Equal(1))
						})
					})

					When("there are no started apps", func() {
						BeforeEach(func() {
							rpcHandlers.GetAppsStub = func(_ string, retVal *[]pluginmodels.GetAppsModel) error {
								*retVal = []pluginmodels.GetAppsModel{}
								return nil
							}
						})

						It("raises an error", func() {
							args := []string{ts.Port(), "restage-all"}
							session, err := gexec.Start(exec.Command(validPluginPath, args...), GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())
							session.Wait()
							Expect(session).To(gbytes.Say("No apps to restage"))
							Expect(session.ExitCode()).To(Equal(1))
						})
					})

					When("build is not staged", func() {
						BeforeEach(func() {
							count := 0
							rpcHandlers.GetOutputAndResetStub = func(b bool, retVal *[]string) error {
								switch count {
								case 0:
									fallthrough
								case 1:
									{
										// /v3/apps/{guid}/droplets/current
										*retVal = []string{`{"created_at": "2020-03-28T23:39:34Z", "links": { "package": { "href": "pkg" } } }`}
									}
								case 2:
									{
										// /v3/builds
										*retVal = []string{`{"guid": "1234"}`}
									}
								default:
									// /v3/builds/{guid}
									*retVal = []string{`{"state": "PENDING", "guid": "1234"}`}
								}
								count++
								return err
							}
						})
						It("should fail to restage application", func() {
							args := []string{ts.Port(), "restage-all", "-stageTimeout=1"}
							session, err := gexec.Start(exec.Command(validPluginPath, args...), GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())
							session.Wait()
							Expect(session).To(gbytes.Say("Starting restage of app1"))
							Expect(session).To(gbytes.Say("Failed to restage application"))
							Expect(session.ExitCode()).To(Equal(0))
						})
					})

					It("restages app1", func() {
						args := []string{ts.Port(), "restage-all"}
						session, err := gexec.Start(exec.Command(validPluginPath, args...), GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())
						session.Wait()
						Expect(session).To(gbytes.Say("Starting restage of app1"))
						Expect(session).To(gbytes.Say("app1 has been restaged successfully"))
						Expect(session.ExitCode()).To(Equal(0))
					})
				})
			})
		})
	})
})

func sampleApps() []pluginmodels.GetAppsModel {
	return []pluginmodels.GetAppsModel{
		{Name: "app1", State: "started", Guid: "1234"},
		//{Name: "app2", State: "started"},
		//{Name: "app3", State: "stopped"},
	}
}
