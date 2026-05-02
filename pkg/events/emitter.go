package events

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
)

const (
	EventScaleUp      = "ScaledUp"
	EventScaleDown    = "ScaledDown"
	EventCooldown     = "Cooldown"
	EventError        = "ScaleError"
	EventMinViolation = "MinViolationCorrected"
	EventMaxViolation = "MaxViolationCorrected"
)

type Emitter struct {
	recorder  record.EventRecorder
	clientset *kubernetes.Clientset
	scheme    *runtime.Scheme
}

func NewEmitter(clientset *kubernetes.Clientset) *Emitter {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	broadcaster := record.NewBroadcaster()
	broadcaster.StartEventWatcher(func(event *corev1.Event) {
		_, _ = clientset.CoreV1().
			Events(event.Namespace).
			Create(context.Background(), event, metav1.CreateOptions{})
	})
	recorder := broadcaster.NewRecorder(scheme, corev1.EventSource{
		Component: "smartscaler-operator",
	})

	return &Emitter{
		recorder:  recorder,
		clientset: clientset,
		scheme:    scheme,
	}
}

func (e *Emitter) Emit(
	namespace string,
	resourceName string,
	eventReason string,
	action string,
	fromReplicas int32,
	toReplicas int32,
	extraInfo string,
) {
	ref := &corev1.ObjectReference{
		Kind:       "SmartScaler",
		APIVersion: "autoscale.mycompany/v1",
		Namespace:  namespace,
		Name:       resourceName,
	}

	eventType := corev1.EventTypeNormal
	if eventReason == EventError {
		eventType = corev1.EventTypeWarning
	}

	msg := fmt.Sprintf("action=%s from=%d to=%d %s", action, fromReplicas, toReplicas, extraInfo)

	e.recorder.Event(ref, eventType, eventReason, msg)
}

func (e *Emitter) EmitError(namespace, resourceName, errMsg string) {
	ref := &corev1.ObjectReference{
		Kind:       "SmartScaler",
		APIVersion: "autoscale.mycompany/v1",
		Namespace:  namespace,
		Name:       resourceName,
	}
	e.recorder.Event(ref, corev1.EventTypeWarning, EventError, errMsg)
}

func DeploymentRef(clientset *kubernetes.Clientset, namespace, name string) (*corev1.ObjectReference, error) {
	deploy, err := clientset.AppsV1().Deployments(namespace).Get(
		context.Background(), name, metav1.GetOptions{},
	)
	if err != nil {
		return nil, err
	}

	scheme := runtime.NewScheme()
	ref, err := reference.GetReference(scheme, deploy)
	if err != nil {
		return nil, err
	}
	return ref, nil
}
