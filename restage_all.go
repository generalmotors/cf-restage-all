package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"strings"
	"time"

	"code.cloudfoundry.org/cli/cf/flags"
	"code.cloudfoundry.org/cli/plugin"
	"github.com/fatih/color"
	"github.com/leaanthony/spinner"
)

type RestageAll struct{}

const (
	restartTimeoutFlag    = "restartTimeout"
	defaultRestartTimeout = 120 // seconds

	stageTimeoutFlag    = "stageTimeout"
	defaultStageTimeout = 120 // seconds

	restageStateFlag    = "state"
	defaultRestageState = "started"

	ageFlag        = "age"
	defaultAgeFlag = 0
)

var (
	stageTimeout   int
	restartTimeout int
	age            int
	restageState   string
)

var (
	red    = color.New(color.FgRed)
	yellow = color.New(color.FgYellow)
)

type Build struct {
	State string `json:"state"`
	Guid  string `json:"guid"`
}

type Application struct {
	State string `json:"state"`
	Guid  string `json:"guid"`
	Name  string `json:"name"`
}

type Droplet struct {
	State     string `json:"state"`
	Name      string `json:"name"`
	CreatedAt string `'json:"created_at"`
}

type CliConnection struct {
	plugin.CliConnection
}

func main() {
	plugin.Start(new(RestageAll))
}

func (c *RestageAll) Run(cliConnection plugin.CliConnection, args []string) {
	switch args[0] {
	case "restage-all":
		fc, err := parseArguments(args)
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			os.Exit(1)
		}
		restartTimeout = fc.Int(restartTimeoutFlag)
		stageTimeout = fc.Int(stageTimeoutFlag)
		restageState = fc.String(restageStateFlag)
		age = fc.Int(ageFlag)
		execute(CliConnection{cliConnection})
	}
}

func parseArguments(args []string) (flags.FlagContext, error) {
	fc := flags.New()
	fc.NewIntFlagWithDefault(restartTimeoutFlag, "rt", "Sets the app restart timeout (seconds)", defaultRestartTimeout)
	fc.NewIntFlagWithDefault(stageTimeoutFlag, "st", "Sets the build restage timeout (seconds)", defaultStageTimeout)
	fc.NewIntFlagWithDefault(ageFlag, "a", "Application age in days - anything older than this will be restaged", defaultAgeFlag)
	fc.NewStringFlagWithDefault(restageStateFlag, "s", "Application state - any apps in this state will be restaged", defaultRestageState)
	err := fc.Parse(args...)
	return fc, err
}

func (c *RestageAll) GetMetadata() plugin.PluginMetadata {

	options := map[string]string{
		"-a": "Restage all applications that contain a droplet older than X days. Default is 0.",
		"-s": "Restage all applications in this state. [started|stopped]. Default is started.",
		"st": "Sets the build restage timeout (seconds) Default is 120.",
		"rt": "Sets the app restart timeout (seconds) Default is 120.",
	}

	return plugin.PluginMetadata{
		Name: "cf-restage-all",
		Version: plugin.VersionType{
			Major: 1,
			Minor: 0,
			Build: 0,
		},
		MinCliVersion: plugin.VersionType{
			Major: 6,
			Minor: 7,
			Build: 0,
		},
		Commands: []plugin.Command{
			{
				Name:     "restage-all",
				HelpText: "Restage applications within a particular space. ",
				UsageDetails: plugin.Usage{
					Usage:   "cf restage-all [--a #] [--s started|stopped] [--rt #] [--st #]",
					Options: options,
				},
			},
		},
	}
}

func execute(cliConnection CliConnection) {

	var err error
	var pkg string
	var restage bool
	var restarted bool

	result, err := cliConnection.GetApps()
	if err != nil {
		exit(err.Error())
	}
	if len(result) == 0 {
		exit("No apps to restage")
	}
	for _, application := range result {
		if application.State != restageState {
			yellow.Printf("Skipping restage on %s in %v state\n", application.Name, application.State)
			continue
		}

		appAge, err := cliConnection.GetAppAge(application.Guid)
		if err != nil {
			red.Printf("Error: %s\n", err.Error())
			continue
		}
		if appAge < age {
			yellow.Printf("Skipping restage on %s as app age is %d \n", application.Name, appAge)
			continue
		} else {
			fmt.Printf("Starting restage of %s\n", application.Name)
		}

		if pkg, err = cliConnection.GetCurrentAppPackage(application.Guid); err != nil {
			red.Printf("Error: %s\n", err.Error())
			continue
		}
		if restage, err = cliConnection.GenerateBuild(application.Guid, pkg); err != nil {
			red.Printf("Error generating build: %s\n", err.Error())
			continue
		}
		if !restage {
			red.Printf("Failed to restage application\n")
			continue
		}

		if restarted, err = cliConnection.RestartApplication(application.Guid); err != nil {
			red.Printf("Error restarting: %s\n", err.Error())
			continue
		}

		if restarted {
			fmt.Printf("%s has been restaged successfully\n", application.Name)
		} else {
			red.Printf("%s has NOT been restaged successfully\n", application.Name)
		}
	}
}

func (cliConnection CliConnection) GetCurrentAppPackage(appGuid string) (string, error) {

	var droplet map[string]interface{}

	packageCurlURL := fmt.Sprintf("/v3/apps/%s/droplets/current", appGuid)
	packageJSON, curlErr := cliConnection.CliCommandWithoutTerminalOutput("curl", "-X", "GET", packageCurlURL)
	if curlErr != nil {
		return "", curlErr
	}

	unmarshallErr := json.Unmarshal([]byte(strings.Join(packageJSON, "")), &droplet)
	if unmarshallErr != nil {
		return "", unmarshallErr
	}
	links := droplet["links"].(map[string]interface{})
	pkg := links["package"].(map[string]interface{})

	guid := fmt.Sprintf("%v", pkg["href"])

	return path.Base(guid), nil
}

func (cliConnection CliConnection) GetAppAge(appGuid string) (int, error) {

	var droplet map[string]interface{}

	packageCurlURL := fmt.Sprintf("/v3/apps/%s/droplets/current", appGuid)
	packageJSON, curlErr := cliConnection.CliCommandWithoutTerminalOutput("curl", "-X", "GET", packageCurlURL)
	if curlErr != nil {
		return 0, curlErr
	}

	unmarshallErr := json.Unmarshal([]byte(strings.Join(packageJSON, "")), &droplet)
	if unmarshallErr != nil {
		return 0, unmarshallErr
	}
	date := droplet["created_at"].(string)

	//2020-11-18T18:40:08Z
	created, e := time.Parse(time.RFC3339, date)
	if e != nil {
		return 0, e
	}

	ageHours := time.Since(created).Hours()

	return int(math.Floor(ageHours / 24)), nil

}

func (cliConnection CliConnection) GetDropletGuid(build Build) (string, error) {

	var results map[string]interface{}

	buildCurlURL := fmt.Sprintf("/v3/builds/%s", build.Guid)
	buildJSON, curlErr := cliConnection.CliCommandWithoutTerminalOutput("curl", "-X", "GET", buildCurlURL)
	if curlErr != nil {
		return "", curlErr
	}

	unmarshallErr := json.Unmarshal([]byte(strings.Join(buildJSON, "")), &results)
	if unmarshallErr != nil {
		return "", unmarshallErr
	}
	droplet := results["droplet"].(map[string]interface{})

	guid := fmt.Sprintf("%v", droplet["guid"])

	return guid, nil
}

func (cliConnection CliConnection) GenerateBuild(appGuid string, pkg string) (bool, error) {

	var build Build
	var restage bool
	var dropletGuid string
	var err error
	var isDropletSet bool

	restageBody := fmt.Sprintf("{ \"package\": {  \"guid\": \"%s\" } }", pkg)
	buildJSON, curlErr := cliConnection.CliCommandWithoutTerminalOutput("curl", "-X", "POST", "/v3/builds", "-d", restageBody)
	if curlErr != nil {
		return false, curlErr
	}

	unmarshallErr := json.Unmarshal([]byte(strings.Join(buildJSON, "")), &build)
	if unmarshallErr != nil {
		return false, unmarshallErr
	}

	if restage, err = cliConnection.WaitForBuildStage(build); err != nil {
		printError(err.Error())
		return false, err
	}

	if !restage {
		return false, nil
	}
	if dropletGuid, err = cliConnection.GetDropletGuid(build); err != nil {
		printError(err.Error())
		return false, err
	}

	if isDropletSet, err = cliConnection.SetDroplet(appGuid, dropletGuid); err != nil {
		printError(err.Error())
		return false, err
	}

	if isDropletSet {
		return true, nil
	} else {
		return false, nil
	}

}

func (cliConnection CliConnection) SetDroplet(appGuid string, droplet string) (bool, error) {

	var isDropletSet bool

	appCurlURL := fmt.Sprintf("/v3/apps/%s/relationships/current_droplet", appGuid)
	appBody := fmt.Sprintf("{ \"data\": {  \"guid\": \"%s\" } }", droplet)
	appJSON, curlErr := cliConnection.CliCommandWithoutTerminalOutput("curl", "-X", "PATCH", appCurlURL, "-d", appBody)
	if curlErr != nil {
		return false, curlErr
	}

	var results map[string]interface{}

	unmarshallErr := json.Unmarshal([]byte(strings.Join(appJSON, "")), &results)
	if unmarshallErr != nil {
		return false, unmarshallErr
	}

	data := results["data"].(map[string]interface{})

	guid := fmt.Sprintf("%v", data["guid"])

	isDropletSet = guid == droplet

	return isDropletSet, nil

}

func (cliConnection CliConnection) WaitForBuildStage(build Build) (bool, error) {
	type Result struct {
		IsStaged bool
		Error    error
	}

	s := spinner.New("Processing Build")
	s.Start()

	c := make(chan Result, 1)
	go func() {
		for {
			isStaged, err := cliConnection.IsBuildStaged(build)
			if err != nil {
				c <- Result{false, err}
			}

			if isStaged {
				c <- Result{true, nil}
				break
			}
			time.Sleep(time.Second)
		}
	}()

	select {
	case res := <-c:
		if res.IsStaged {
			s.Success("Build created")
		} else {
			s.Error("Build failed")
		}
		return res.IsStaged, res.Error
	case <-time.After(time.Duration(stageTimeout) * time.Second):
		s.Error("Timed out waiting for build")
		return false, nil
	}
}

func (cliConnection CliConnection) GetBuildInfo(build Build) (Build, error) {

	buildCurlURL := fmt.Sprintf("/v3/builds/%s", build.Guid)
	buildJSON, curlErr := cliConnection.CliCommandWithoutTerminalOutput("curl", "-X", "GET", buildCurlURL)
	if curlErr != nil {
		return build, curlErr
	}

	unmarshallErr := json.Unmarshal([]byte(strings.Join(buildJSON, "")), &build)
	if unmarshallErr != nil {
		return build, unmarshallErr
	}

	return build, nil
}

func (cliConnection CliConnection) IsBuildStaged(build Build) (bool, error) {
	var isStaged bool
	var newBuild Build
	var err error

	if newBuild, err = cliConnection.GetBuildInfo(build); err != nil {
		return false, err
	}

	isStaged = newBuild.State == "STAGED"

	return isStaged, nil
}

func (cliConnection CliConnection) RestartApplication(appGuid string) (bool, error) {
	appCurlURL := fmt.Sprintf("/v3/apps/%s/actions/restart", appGuid)
	appJSON, curlErr := cliConnection.CliCommandWithoutTerminalOutput("curl", "-X", "POST", appCurlURL)
	if curlErr != nil {
		return false, curlErr
	}

	var result map[string]interface{}

	unmarshallErr := json.Unmarshal([]byte(strings.Join(appJSON, "")), &result)
	if unmarshallErr != nil {
		return false, unmarshallErr
	}

	return cliConnection.WaitForApplicationStart(appGuid)
}

func (cliConnection CliConnection) WaitForApplicationStart(appGuid string) (bool, error) {
	type Result struct {
		IsStarted bool
		Error     error
	}

	s := spinner.New("Restarting Application")
	s.Start()

	c := make(chan Result, 1)
	go func() {
		for {
			isStarted, err := cliConnection.IsApplicationStarted(appGuid)
			if err != nil {
				c <- Result{false, err}
			}

			if isStarted {
				c <- Result{true, nil}
				break
			}
			time.Sleep(time.Second)
		}
	}()

	select {
	case res := <-c:
		if res.IsStarted {
			s.Success("Application Restarted")
		} else {
			s.Error("Failed to restart application")
		}
		return res.IsStarted, res.Error
	case <-time.After(time.Duration(restartTimeout) * time.Second):
		s.Error("Timed out waiting for application restart")
		return false, nil
	}
}

func (cliConnection CliConnection) IsApplicationStarted(appGuid string) (bool, error) {
	app, err := cliConnection.GetAppInfo(appGuid)
	if err != nil {
		return false, err
	}
	return app.State == "STARTED", nil
}

func (cliConnection CliConnection) GetAppInfo(appGuid string) (app Application, err error) {
	url := fmt.Sprintf("/v3/apps/%s", appGuid)
	body, curlErr := cliConnection.CliCommandWithoutTerminalOutput("curl", "-X", "GET", url)
	if curlErr != nil {
		return app, curlErr
	}

	err = json.Unmarshal([]byte(strings.Join(body, "")), &app)
	if err != nil {
		return app, err
	}

	return app, nil
}

func printError(message string) {
	fmt.Printf("Error: %s\n", message)
}

func exit(err string) {
	fmt.Printf("Fatal Error: %s\n", err)
	os.Exit(1)
}
