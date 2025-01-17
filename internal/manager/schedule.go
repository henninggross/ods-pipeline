package manager

import (
	"context"
	"strings"
	"time"

	kubernetesClient "github.com/opendevstack/pipeline/internal/kubernetes"
	tektonClient "github.com/opendevstack/pipeline/internal/tekton"
	"github.com/opendevstack/pipeline/pkg/config"
	"github.com/opendevstack/pipeline/pkg/logging"
	tekton "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StorageConfig struct {
	Provisioner string
	ClassName   string
	Size        string
}

// Scheduler creates or updates pipelines based on PipelineConfig received from
// the TriggeredPipelines channel. It then schedules a pipeline run
// connected to the pipeline. If the run cannot start immediately because
// of another run, the new pipeline run is created in pending status.
type Scheduler struct {
	// Channel to read newly received runs from
	TriggeredPipelines chan PipelineConfig
	// Channel to send pending runs on
	PendingRunRepos  chan string
	TektonClient     tektonClient.ClientInterface
	KubernetesClient kubernetesClient.ClientInterface
	Logger           logging.LeveledLoggerInterface
	// TaskKind is the Tekton resource kind for tasks.
	// Either "ClusterTask" or "Task".
	TaskKind tekton.TaskKind
	// TaskSuffic is the suffix applied to tasks (version information).
	TaskSuffix string

	StorageConfig StorageConfig
}

// Run starts the scheduling process.
func (s *Scheduler) Run(ctx context.Context) {
	for {
		select {
		case pData := <-s.TriggeredPipelines:
			needQueueing := s.schedule(ctx, pData)
			if needQueueing {
				s.PendingRunRepos <- pData.Repository
			}
		case <-ctx.Done():
			return
		}
	}
}

// schedule turns a PipelineConfig into a pipeline (run).
func (s *Scheduler) schedule(ctx context.Context, pData PipelineConfig) bool {
	ctxt, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	newPipeline := assemblePipeline(pData, s.TaskKind, s.TaskSuffix)

	existingPipeline, err := s.TektonClient.GetPipeline(ctxt, pData.Name, metav1.GetOptions{})
	if err != nil {
		_, err := s.TektonClient.CreatePipeline(ctxt, newPipeline, metav1.CreateOptions{})
		if err != nil {
			s.Logger.Errorf(err.Error())
			return false
		}
	} else {
		newPipeline.ResourceVersion = existingPipeline.ResourceVersion
		_, err := s.TektonClient.UpdatePipeline(ctxt, newPipeline, metav1.UpdateOptions{})
		if err != nil {
			s.Logger.Errorf(err.Error())
			return false
		}
	}

	// Create PVC if it does not exist yet
	err = s.createPVCIfRequired(ctxt, pData)
	if err != nil {
		s.Logger.Errorf(err.Error())
		return false
	}

	pipelineRuns, err := listPipelineRuns(ctxt, s.TektonClient, pData.Repository)
	if err != nil {
		s.Logger.Errorf(err.Error())
		return false
	}
	s.Logger.Debugf("Found %d pipeline runs related to repository %s.", len(pipelineRuns.Items), pData.Repository)
	needQueueing := needsQueueing(pipelineRuns)
	s.Logger.Debugf("Creating run for pipeline %s (queued=%v) ...", pData.Name, needQueueing)
	_, err = createPipelineRun(s.TektonClient, ctxt, pData, needQueueing)
	if err != nil {
		s.Logger.Errorf(err.Error())
		return false
	}
	return needQueueing
}

// needsQueueing checks if any run has either:
// - pending status set OR
// - is progressing
func needsQueueing(pipelineRuns *tekton.PipelineRunList) bool {
	for _, pr := range pipelineRuns.Items {
		if pr.Spec.Status == tekton.PipelineRunSpecStatusPending || pipelineRunIsProgressing(pr) {
			return true
		}
	}
	return false
}

// selectEnvironmentFromMapping selects the environment name matching given branch.
func selectEnvironmentFromMapping(mapping []config.BranchToEnvironmentMapping, branch string) string {
	for _, bem := range mapping {
		if mappingBranchMatch(bem.Branch, branch) {
			return bem.Environment
		}
	}
	return ""
}

func mappingBranchMatch(mappingBranch, testBranch string) bool {
	// exact match
	if mappingBranch == testBranch {
		return true
	}
	// prefix match like "release/*", also catches "*"
	if strings.HasSuffix(mappingBranch, "*") {
		branchPrefix := strings.TrimSuffix(mappingBranch, "*")
		if strings.HasPrefix(testBranch, branchPrefix) {
			return true
		}
	}
	return false
}
