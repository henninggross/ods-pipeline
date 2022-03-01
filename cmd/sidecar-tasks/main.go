package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	tekton "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func main() {
	tasksWithSidecars := []string{
		"ods-build-go",
		"ods-build-gradle",
		"ods-build-python",
		"ods-build-typescript",
	}
	t, err := parseTasks(tasksWithSidecars)
	if err != nil {
		log.Fatal(err)
	}
	adjustTasks(t)
	err = writeTasks(t)
	if err != nil {
		log.Fatal(err)
	}
}

func parseTasks(taskNames []string) (map[string]*tekton.Task, error) {
	tasks := map[string]*tekton.Task{}
	for _, task := range taskNames {
		fmt.Printf("Parsing task %s ...\n", task)
		b, err := ioutil.ReadFile(fmt.Sprintf("deploy/ods-pipeline/charts/ods-pipeline-tasks/templates/task-%s.yaml", task))
		if err != nil {
			return nil, err
		}
		var t tekton.Task
		err = yaml.Unmarshal(b, &t)
		if err != nil {
			return nil, err
		}
		tasks[task] = &t
	}
	return tasks, nil
}

func adjustTasks(tasks map[string]*tekton.Task) {
	for name, t := range tasks {
		fmt.Printf("Adding sidecar to task %s ...\n", name)
		cleanName := strings.Replace(t.Name, "{{default \"ods\" .Values.taskPrefix}}", "ods", 1)
		cleanName = strings.Replace(cleanName, "{{.Values.global.taskSuffix}}", "", 1)
		t.Name = strings.Replace(t.Name, "{{.Values.global.taskSuffix}}", "-with-sidecar{{.Values.global.taskSuffix}}", 1)
		t.Spec.Description = t.Spec.Description + `
**Sidecar variant!** Use this task if you need to run a container next to the build task.
For example, this could be used to run a database to allow for integration tests.
The sidecar image to must be supplied via ` + "`sidecar-image`" + `.
Apart from the sidecar, the task is an exact copy of ` + "`" + cleanName + "`" + `.`
		t.Spec.Params = append(t.Spec.Params, tekton.ParamSpec{
			Name:        "sidecar-image",
			Description: "Image to use for sidecar",
			Type:        tekton.ParamTypeString,
		})
		t.Spec.Sidecars = []tekton.Sidecar{
			{
				Container: corev1.Container{
					Name:  "sidecar",
					Image: "$(params.sidecar-image)",
				},
			},
		}
	}
}

func writeTasks(tasks map[string]*tekton.Task) error {
	for name, t := range tasks {
		fmt.Printf("Writing sidecar task %s ...\n", name)
		out, err := yaml.Marshal(t)
		if err != nil {
			return err
		}
		out = append([]byte("# Generated by cmd/sidecar-tasks/main.go; DO NOT EDIT.\n"), out...)
		err = ioutil.WriteFile(
			fmt.Sprintf("deploy/ods-pipeline/charts/ods-pipeline-tasks/templates/task-%s-with-sidecar.yaml", name),
			out, 0644,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
