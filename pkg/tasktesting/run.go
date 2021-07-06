package tasktesting

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opendevstack/pipeline/internal/directory"
	"github.com/opendevstack/pipeline/internal/kubernetes"
	"github.com/opendevstack/pipeline/internal/projectpath"
	"github.com/opendevstack/pipeline/pkg/pipelinectxt"
)

const (
	namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

type TestOpts struct {
	TaskKindRef             string
	TaskName                string
	Clients                 *kubernetes.Clients
	Namespace               string
	Timeout                 time.Duration
	AlwaysKeepTmpWorkspaces bool
}

type TestCase struct {
	// Map workspace name of task to local directory under test/testdata/workspaces.
	WorkspaceDirMapping map[string]string
	WantRunSuccess      bool
	PreRunFunc          func(t *testing.T, ctxt *TaskRunContext)
	PostRunFunc         func(t *testing.T, ctxt *TaskRunContext)
}

type TaskRunContext struct {
	Namespace  string
	Clients    *kubernetes.Clients
	Workspaces map[string]string
	Params     map[string]string
	ODS        *pipelinectxt.ODSContext
}

func Run(t *testing.T, tc TestCase, testOpts TestOpts) {

	// Set default timeout for running the test
	if testOpts.Timeout == 0 {
		testOpts.Timeout = 120 * time.Second
	}

	taskWorkspaces := map[string]string{}
	for wn, wd := range tc.WorkspaceDirMapping {
		tempDir, err := InitWorkspace(wn, wd)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Workspace is in %s", tempDir)
		taskWorkspaces[wn] = tempDir
	}

	testCaseContext := &TaskRunContext{
		Namespace:  testOpts.Namespace,
		Clients:    testOpts.Clients,
		Workspaces: taskWorkspaces,
	}

	if tc.PreRunFunc != nil {
		tc.PreRunFunc(t, testCaseContext)
	}

	tr, err := CreateTaskRunWithParams(
		testOpts.Clients.TektonClientSet,
		testOpts.TaskKindRef,
		testOpts.TaskName,
		testCaseContext.Params,
		taskWorkspaces,
		testOpts.Namespace,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for pod to exist.
	// Listen for events, and while waiting, check that pod is running.
	// Once the pod is running,stop listening for events and collect logs.
	pod := WaitForTaskRunPod(t, testOpts.Clients.KubernetesClientSet, tr.Name, testOpts.Namespace)

	//podEventsDone := make(chan bool, 1)
	//go WatchTaskRunEvents(t, testOpts.Clients.KubernetesClientSet, tr.Name, testOpts.Namespace, podEventsDone)
	quitEvents := make(chan bool, 1)
	go WatchPodEvents(t, testOpts.Clients.KubernetesClientSet, pod.Name, testOpts.Namespace, quitEvents)

	err = getLogs(testOpts.Clients.KubernetesClientSet, pod, quitEvents)
	if err != nil {
		t.Fatal(err)
	}

	// Wait X minutes for task to complete or be notified of a failure from an pods' event
	tr = WaitForCondition(context.TODO(), t, testOpts.Clients.TektonClientSet, tr.Name, testOpts.Namespace, Done, testOpts.Timeout, quitEvents)

	// Show logs
	// go CollectPodLogs(testOpts.Clients.KubernetesClientSet, tr.Status.PodName, testOpts.Namespace, t.Logf, podEventsDone)

	// Block until we receive a notification from CollectPodLogs on the channel
	//<-podEventsDone

	// Show info from Task result
	CollectTaskResultInfo(tr, t.Logf)

	// Check if task was successful
	if tr.IsSuccessful() != tc.WantRunSuccess {
		t.Fatalf("Got: %+v, want: %+v.", tr.IsSuccessful(), tc.WantRunSuccess)
	}

	// Check local folder and evaluate output of task if needed
	if tc.PostRunFunc != nil {
		tc.PostRunFunc(t, testCaseContext)
	}

	if !testOpts.AlwaysKeepTmpWorkspaces {
		// Clean up only if test is successful
		for _, wd := range taskWorkspaces {
			err = os.RemoveAll(wd)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func InitWorkspace(workspaceName, workspaceDir string) (string, error) {
	workspaceSourceDirectory := filepath.Join(
		projectpath.Root, "test", testdataWorkspacePath, workspaceDir,
	)

	workspaceParentDirectory := filepath.Dir(workspaceSourceDirectory)

	tempDir, err := ioutil.TempDir(workspaceParentDirectory, "workspace-")
	if err != nil {
		return "", err
	}

	directory.Copy(workspaceSourceDirectory, tempDir)

	return tempDir, nil
}
