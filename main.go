package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func main() {
	var kubeconfig string
	flag.StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig file")
	flag.Parse()

	config, err := buildConfig(kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to build config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create clientset: %v", err)
	}

	klog.Info("Starting pod recycle controller")
	watchPods(clientset)
}

func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

func watchPods(clientset *kubernetes.Clientset) {
	for {
		watcher, err := clientset.CoreV1().Pods("").Watch(context.Background(), metav1.ListOptions{})
		if err != nil {
			klog.Errorf("Failed to watch pods: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for event := range watcher.ResultChan() {
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			// Skip if pod is already being deleted
			if pod.DeletionTimestamp != nil {
				continue
			}

			if event.Type == watch.Modified && isInCrashLoopBackOff(pod) {
				klog.Infof("Detected CrashLoopBackOff for pod %s/%s, force deleting", pod.Namespace, pod.Name)
				if err := forceDeletePod(clientset, pod); err != nil {
					klog.Errorf("Failed to delete pod %s/%s: %v", pod.Namespace, pod.Name, err)
				}
			}
		}

		klog.Warning("Watch connection closed, reconnecting...")
		time.Sleep(5 * time.Second)
	}
}

func isInCrashLoopBackOff(pod *corev1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil &&
			containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
			return true
		}
	}
	return false
}

func forceDeletePod(clientset *kubernetes.Clientset, pod *corev1.Pod) error {
	gracePeriod := int64(0)
	deleteOptions := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	}

	err := clientset.CoreV1().Pods(pod.Namespace).Delete(
		context.Background(),
		pod.Name,
		deleteOptions,
	)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	klog.Infof("Successfully deleted pod %s/%s", pod.Namespace, pod.Name)
	return nil
}
