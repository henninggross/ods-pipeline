package tasks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opendevstack/pipeline/pkg/pipelinectxt"

	"github.com/opendevstack/pipeline/pkg/tasktesting"
)

func TestTaskODSBuildGradle(t *testing.T) {
	runTaskTestCases(t,
		"ods-build-gradle",
		[]tasktesting.Service{
			tasktesting.Bitbucket,
			tasktesting.Nexus,
			tasktesting.SonarQube,
		},
		map[string]tasktesting.TestCase{
			"task should build gradle app": {
				WorkspaceDirMapping: map[string]string{"source": "gradle-sample-app"},
				PreRunFunc: func(t *testing.T, ctxt *tasktesting.TaskRunContext) {
					wsDir := ctxt.Workspaces["source"]
					ctxt.ODS = tasktesting.SetupGitRepo(t, ctxt.Namespace, wsDir)
					ctxt.Params = map[string]string{
						"sonar-quality-gate": "true",
					}
				},
				WantRunSuccess: true,
				PostRunFunc: func(t *testing.T, ctxt *tasktesting.TaskRunContext) {
					wsDir := ctxt.Workspaces["source"]

					wantFiles := []string{
						"docker/Dockerfile",
						"docker/app.jar",
						filepath.Join(pipelinectxt.XUnitReportsPath, "report.xml"),
						filepath.Join(pipelinectxt.CodeCoveragesPath, "coverage.xml"),
						filepath.Join(pipelinectxt.SonarAnalysisPath, "analysis-report.md"),
						filepath.Join(pipelinectxt.SonarAnalysisPath, "issues-report.csv"),
						filepath.Join(pipelinectxt.SonarAnalysisPath, "quality-gate.json"),
					}
					for _, wf := range wantFiles {
						if _, err := os.Stat(filepath.Join(wsDir, wf)); os.IsNotExist(err) {
							t.Fatalf("Want %s, but got nothing", wf)
						}
					}
				},
			},
		})
}
