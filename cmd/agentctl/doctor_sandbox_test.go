package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRunSandboxDoctor(t *testing.T) {
	tests := []struct {
		name      string
		objects   []runtime.Object
		want      string
		wantError bool
	}{
		{name: "missing runtime class", want: "RuntimeClass gvisor: missing\nReady nodes matching RuntimeClass: 0\nFAIL: no RuntimeClass named \"gvisor\"\n", wantError: true},
		{name: "no ready node", objects: []runtime.Object{doctorRuntimeClass()}, want: "RuntimeClass gvisor: found\nReady nodes matching RuntimeClass: 0\nFAIL: no Ready nodes match the RuntimeClass\n", wantError: true},
		{name: "ready runtime class", objects: []runtime.Object{doctorRuntimeClass(), doctorReadyNode()}, want: "RuntimeClass gvisor: found\nReady nodes matching RuntimeClass: 1\nPASS\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := runSandboxDoctor(context.Background(), doctorReader(t, test.objects...), "gvisor", &output)
			if (err != nil) != test.wantError {
				t.Fatalf("runSandboxDoctor() error = %v, wantError %t", err, test.wantError)
			}
			if output.String() != test.want {
				t.Fatalf("output = %q, want %q", output.String(), test.want)
			}
		})
	}
}

func TestNewSandboxDoctorCommandDefaultsToOperatorRuntimeClass(t *testing.T) {
	t.Setenv("RUNTIME_SANDBOX_CLASS", "kata")
	command := newSandboxDoctorCommand(&cliOptions{})
	if got := command.Flags().Lookup("runtime-class").DefValue; got != "kata" {
		t.Fatalf("runtime-class default = %q, want kata", got)
	}
}

func TestRunSandboxDoctorReportsAPIFailure(t *testing.T) {
	var output bytes.Buffer
	err := runSandboxDoctor(context.Background(), doctorFailingReader{}, "gvisor", &output)
	if err == nil {
		t.Fatal("runSandboxDoctor() error = nil, want API error")
	}
	want := "RuntimeClass gvisor: unknown\nReady nodes matching RuntimeClass: 0\nFAIL: sandbox readiness check failed\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestRunSandboxDoctorReportsRuntimeClassWhenNodeCheckFails(t *testing.T) {
	reader := doctorReader(t, doctorRuntimeClass())
	var output bytes.Buffer
	err := runSandboxDoctor(context.Background(), doctorNodeFailingReader{Reader: reader}, "gvisor", &output)
	if err == nil {
		t.Fatal("runSandboxDoctor() error = nil, want API error")
	}
	want := "RuntimeClass gvisor: found (node check failed)\nReady nodes matching RuntimeClass: 0\nFAIL: sandbox readiness check failed\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func doctorReader(t *testing.T, objects ...runtime.Object) client.Reader {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := nodev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
}

func doctorRuntimeClass() *nodev1.RuntimeClass {
	return &nodev1.RuntimeClass{ObjectMeta: metav1.ObjectMeta{Name: "gvisor"}, Handler: "runsc"}
}

func doctorReadyNode() *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{"agentic.clawdlinux.org/gvisor-ready": "true"}}, Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}}}
}

var errDoctorAPI = errors.New("API unavailable")

type doctorFailingReader struct{}

func (doctorFailingReader) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	return errDoctorAPI
}

func (doctorFailingReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return errDoctorAPI
}

type doctorNodeFailingReader struct {
	client.Reader
}

func (doctorNodeFailingReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return errDoctorAPI
}

func TestNewDoctorCommandIncludesSandbox(t *testing.T) {
	command := newDoctorCommand(&cliOptions{})
	child, _, err := command.Find([]string{"sandbox"})
	if err != nil || child == nil || child.Name() != "sandbox" {
		t.Fatalf("sandbox command = %v, %v, want sandbox", child, err)
	}
	if !strings.Contains(command.Use, "doctor") {
		t.Fatalf("doctor command Use = %q", command.Use)
	}
}
