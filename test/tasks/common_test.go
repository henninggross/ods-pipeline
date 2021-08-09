package tasks

import (
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/opendevstack/pipeline/internal/kubernetes"
	"github.com/opendevstack/pipeline/pkg/bitbucket"
	"github.com/opendevstack/pipeline/pkg/config"
	"github.com/opendevstack/pipeline/pkg/pipelinectxt"
	"github.com/opendevstack/pipeline/pkg/sonar"
	"github.com/opendevstack/pipeline/pkg/tasktesting"
	kclient "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

var alwaysKeepTmpWorkspacesFlag = flag.Bool("always-keep-tmp-workspaces", false, "Whether to keep temporary workspaces from taskruns even when test is successful")

const (
	bitbucketProjectKey = "ODSPIPELINETEST"
	taskKindRef         = "ClusterTask"
	storageClasName     = "standard" // if using KinD, set it to "standard"
	storageCapacity     = "1Gi"
	storageSourceDir    = "/files" // this is the dir *within* the KinD container that mounts to ${ODS_PIPELINE_DIR}/test
)

func checkODSContext(t *testing.T, repoDir string, want *pipelinectxt.ODSContext) {
	checkODSFileContent(t, repoDir, "component", want.Component)
	checkODSFileContent(t, repoDir, "git-commit-sha", want.GitCommitSHA)
	checkODSFileContent(t, repoDir, "git-full-ref", want.GitFullRef)
	checkODSFileContent(t, repoDir, "git-ref", want.GitRef)
	checkODSFileContent(t, repoDir, "git-url", want.GitURL)
	checkODSFileContent(t, repoDir, "namespace", want.Namespace)
	checkODSFileContent(t, repoDir, "pr-base", want.PullRequestBase)
	checkODSFileContent(t, repoDir, "pr-key", want.PullRequestKey)
	checkODSFileContent(t, repoDir, "project", want.Project)
	checkODSFileContent(t, repoDir, "repository", want.Repository)
}

func checkODSFileContent(t *testing.T, wsDir, filename, want string) {
	checkFileContent(t, filepath.Join(wsDir, pipelinectxt.BaseDir), filename, want)
}

func checkFileContent(t *testing.T, wsDir, filename, want string) {
	got, err := getTrimmedFileContent(filepath.Join(wsDir, filename))
	if err != nil {
		t.Fatalf("could not read %s: %s", filename, err)
	}
	if got != want {
		t.Fatalf("got '%s', want '%s' in file %s", got, want, filename)
	}
}

func getTrimmedFileContent(filename string) (string, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func trimmedFileContentOrFatal(t *testing.T, filename string) string {
	c, err := getTrimmedFileContent(filename)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func checkFileContentContains(t *testing.T, wsDir, filename, wantContains string) {
	got, err := getFileContentLean(filepath.Join(wsDir, filename))
	if err != nil {
		t.Fatalf("could not read %s: %s", filename, err)
	}
	if !strings.Contains(got, wantContains) {
		t.Fatalf("got '%s', wantContains '%s' in file %s", got, wantContains, filename)
	}
}

func getFileContentLean(filename string) (string, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}

	contentStr := strings.ReplaceAll(string(content), "\t", "")
	contentStr = strings.ReplaceAll(contentStr, "\n", "")
	contentStr = strings.ReplaceAll(contentStr, " ", "")

	return contentStr, nil
}

func runTaskTestCases(t *testing.T, taskName string, testCases map[string]tasktesting.TestCase) {
	c, ns := tasktesting.Setup(t,
		tasktesting.SetupOpts{
			SourceDir:        storageSourceDir,
			StorageCapacity:  storageCapacity,
			StorageClassName: storageClasName,
		},
	)

	tasktesting.CleanupOnInterrupt(func() { tasktesting.TearDown(t, c, ns) }, t.Logf)
	defer tasktesting.TearDown(t, c, ns)

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			tasktesting.Run(t, tc, tasktesting.TestOpts{
				TaskKindRef:             taskKindRef, // could be read from task definition
				TaskName:                taskName,    // could be read from task definition
				Clients:                 c,
				Namespace:               ns,
				Timeout:                 5 * time.Minute, // depending on  the task we may need to increase or decrease it
				AlwaysKeepTmpWorkspaces: *alwaysKeepTmpWorkspacesFlag,
			})
			t.Logf("Test execution time: %fs", time.Since(start).Seconds())
		})
	}
}

func checkSonarQualityGate(t *testing.T, c *kclient.Clientset, ctxt *tasktesting.TaskRunContext, qualityGateFlag bool, wantQualityGateStatus string) {

	sonarToken, err := kubernetes.GetSecretKey(c, ctxt.Namespace, "ods-sonar-auth", "password")
	if err != nil {
		t.Fatalf("could not get SonarQube token: %s", err)
	}

	sonarClient := sonar.NewClient(&sonar.ClientConfig{
		APIToken:      sonarToken,
		BaseURL:       "http://localhost:9000", // use localhost instead of sonarqubetest.kind!
		ServerEdition: "community",
	})

	if qualityGateFlag {
		sonarProject := fmt.Sprintf("%s-%s", ctxt.ODS.Project, ctxt.ODS.Component)
		qualityGateResult, err := sonarClient.QualityGateGet(
			sonar.QualityGateGetParams{Project: sonarProject},
		)
		if err != nil || qualityGateResult.ProjectStatus.Status == "UNKNOWN" {
			t.Log("quality gate unknown")
			t.Fatal(err)
		}

		if qualityGateResult.ProjectStatus.Status != wantQualityGateStatus {
			t.Fatalf("Got: %s, want: %s", qualityGateResult.ProjectStatus.Status, wantQualityGateStatus)
		}

	}

}

func createODSYML(wsDir string, o *config.ODS) error {
	y, err := yaml.Marshal(o)
	if err != nil {
		return err
	}
	filename := filepath.Join(wsDir, "ods.yml")
	return ioutil.WriteFile(filename, y, 0644)
}

func checkBuildStatus(t *testing.T, c *bitbucket.Client, gitCommit, wantBuildStatus string) {
	buildStatus, err := c.BuildStatusGet(gitCommit)
	if err != nil {
		t.Fatal(err)
	}
	if buildStatus.State != wantBuildStatus {
		t.Fatalf("Got: %s, want: %s", buildStatus.State, wantBuildStatus)
	}

}
