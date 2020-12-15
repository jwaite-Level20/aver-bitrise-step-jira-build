package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Holdapp/bitrise-step-jira-build/bitrise"
	"github.com/Holdapp/bitrise-step-jira-build/config"
	"github.com/Holdapp/bitrise-step-jira-build/service"

	"github.com/bitrise-io/go-steputils/stepconf"
)

type StepConfig struct {
	// Generar info
	AppVersion string `env:"APP_VERSION,required"`

	// JIRA
	JiraHost     string          `env:"JIRA_HOST,required"`
	JiraUsername string          `env:"JIRA_USERNAME,required"`
	JiraToken    stepconf.Secret `env:"JIRA_ACCESS_TOKEN,required"`
	JiraFieldID  int             `env:"JIRA_CUSTOM_FIELD_ID,required"`

	// Bitrise API
	BitriseToken stepconf.Secret `env:"BITRISE_API_TOKEN,required"`

	// Fields provided by Bitrise
	BuildNumber string `env:"BITRISE_BUILD_NUMBER,required"`
	Workflow    string `env:"BITRISE_TRIGGERED_WORKFLOW_TITLE,required"`
	SourceDir   string `env:"BITRISE_SOURCE_DIR,required"`
	Branch      string `env:"BITRISE_GIT_BRANCH,required"`
	BuildSlug   string `env:"BITRISE_BUILD_SLUG,required"`
	AppSlug     string `env:"BITRISE_APP_SLUG,required"`
}

func (config *StepConfig) JiraTokenString() string {
	return string(config.JiraToken)
}

func (config *StepConfig) BitriseTokenString() string {
	return string(config.BitriseToken)
}

func main() {
	// Parse config
	var stepConfig = StepConfig{}
	if err := stepconf.Parse(&stepConfig); err != nil {
		log.Fatal(err)
	}

	build := config.Build{
		Version: stepConfig.AppVersion,
		Number:  stepConfig.BuildNumber,
	}

	// get commit hashes from bitrise
	fmt.Println("Scanning Bitrise API for previous failed/aborted builds")
	bitriseClient := bitrise.Client{Token: stepConfig.BitriseTokenString()}
	hashes, err := service.ScanRelatedCommits(
		&bitriseClient, stepConfig.AppSlug,
		stepConfig.BuildSlug, stepConfig.Workflow,
		stepConfig.Branch,
	)
	if err != nil {
		log.Fatal(err)
	}

	// scan repo for related issue keys
	fmt.Printf("Scanning git repo for JIRA issues (%d anchor[s])\n", len(hashes))
	gitWorker, err := service.GitOpen(stepConfig.SourceDir, stepConfig.Branch, hashes)
	if err != nil {
		log.Fatal(err)
	}

	issueKeys := gitWorker.ScanIssues()

	// update custom field on issues with current build number
	fmt.Printf("Updating build status for issues: %v\n", issueKeys)
	jiraWorker, err := service.NewJIRAWorker(
		stepConfig.JiraHost, stepConfig.JiraUsername,
		stepConfig.JiraTokenString(), stepConfig.JiraFieldID,
	)
	if err != nil {
		log.Fatalln(err)
	}

	jiraWorker.UpdateBuildForIssues(issueKeys, build)

	// exit with success code
	os.Exit(0)
}
