package helpers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSelectPortForwardPodPrefersRunningReadyPod(t *testing.T) {
	now := metav1.NewTime(time.Now())

	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "spectre-old",
				DeletionTimestamp: &now,
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "spectre-new"},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				},
			},
		},
	}

	podName, err := selectPortForwardPodName(pods)
	require.NoError(t, err)
	require.Equal(t, "spectre-new", podName)
}

func TestSelectPortForwardPodRejectsPodsThatAreNotReady(t *testing.T) {
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "spectre-pending"},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "spectre-running-not-ready"},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		},
	}

	_, err := selectPortForwardPodName(pods)
	require.Error(t, err)
}
