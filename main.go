package main

// https://itnext.io/generically-working-with-kubernetes-resources-in-go-53bce678f887

import (
	"context"
	"fmt"
	"github.com/itchyny/gojq"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
)

func main() {
	ctx := context.Background()
	config := ctrl.GetConfigOrDie()
	clientset := kubernetes.NewForConfigOrDie(config)
	dynamic := dynamic.NewForConfigOrDie(config)

	args := os.Args

	namespace := "resiliency-dev"
	getDeployments(args, clientset, dynamic, ctx, namespace)
}

func getDeployments(args []string, clientset *kubernetes.Clientset, dynamic dynamic.Interface, ctx context.Context, namespace string) {
	if len(args) != 2 {
		fmt.Println("Usage: go run ./main.go <approach>")
		fmt.Println("\tapproach: clientset | dynamic | dynamic-jq | clientset-jq")
		return
	}

	if args[1] == "clientset" {
		fmt.Println("using client set")
		items, err := GetDeployments(clientset, ctx, namespace)

		if err != nil {
			fmt.Println(err)
		} else {
			for _, item := range items {
				fmt.Printf("%+v\n", item)
			}
		}
	} else if args[1] == "dynamic" {
		fmt.Println("using dynamic")
		items, err := GetResourcesDynamically(dynamic, ctx, "apps", "v1", "deployments", namespace)

		if err != nil {
			fmt.Println(err)
		} else {
			for _, item := range items {
				fmt.Printf("%+v\n", item)
			}
		}
	} else if args[1] == "dynamic-jq" {
		fmt.Println("using dynamic-jq")
		query := ".metadata.labels[\"app.kubernetes.io/managed-by\"] == \"Helm\""
		// query := ".metadata.labels[\"app.kubernetes.io/managed-by\"] == \"HelmABC\"" // purposefully wrong
		// if query is not found, nothing is returned and no error is thrown, nothing prints
		items, err := GetResourcesDynamicallyByJq(dynamic, ctx, "apps", "v1", "deployments", namespace, query)
		if err != nil {
			fmt.Println(err)
		} else {
			for _, item := range items {
				fmt.Printf("%+v\n", item)
			}
		}
	} else if args[1] == "clientset-jq" {
		fmt.Println("using clientset-jq")
		key := "app.kubernetes.io/managed-by"
		value := "Helm"
		items, err := GetDeploymentsByJq(clientset, ctx, namespace, key, value)
		if err != nil {
			fmt.Println(err)
		} else {
			for _, item := range items {
				fmt.Printf("%+v\n", item)
			}
		}
	} else {
		fmt.Println("Usage: go run ./main.go <approach>")
		fmt.Println("\tapproach: clientset | dynamic | dynamic-jq | clientset-jq")
		fmt.Println("\tappproach not found use clientset | dynamic | dynamic-jq | clientset-jq")
		fmt.Println("\tapproach passed in:", args[1])
		return
	}
}

func GetDeployments(clientset *kubernetes.Clientset, ctx context.Context,
	namespace string) ([]v1.Deployment, error) {

	list, err := clientset.AppsV1().Deployments(namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func GetResourcesDynamically(dynamic dynamic.Interface, ctx context.Context,
	group string, version string, resource string, namespace string) (
	[]unstructured.Unstructured, error) {

	resourceId := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
	list, err := dynamic.Resource(resourceId).Namespace(namespace).
		List(ctx, metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func GetResourcesDynamicallyByJq(dynamic dynamic.Interface, ctx context.Context, group string,
	version string, resource string, namespace string, jq string) (
	[]unstructured.Unstructured, error) {

	resources := make([]unstructured.Unstructured, 0)

	query, err := gojq.Parse(jq)
	if err != nil {
		return nil, err
	}

	items, err := GetResourcesDynamically(dynamic, ctx, group, version, resource, namespace)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		// Convert object to raw JSON
		var rawJson interface{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &rawJson)
		if err != nil {
			return nil, err
		}

		// Evaluate jq against JSON
		iter := query.Run(rawJson)
		for {
			result, ok := iter.Next()
			if !ok {
				break
			}
			if err, ok := result.(error); ok {
				if err != nil {
					return nil, err
				}
			} else {
				boolResult, ok := result.(bool)
				fmt.Println("boolResult:", boolResult, "ok:", ok)
				if !ok {
					fmt.Println("Query returned non-boolean value")
				} else if boolResult {
					resources = append(resources, item)
				}
			}
		}
	}

	return resources, nil
}

func GetDeploymentsByJq(clientset *kubernetes.Clientset, ctx context.Context,
	namespace, key, value string) ([]v1.Deployment, error) {

	resources := make([]v1.Deployment, 0)
	items, err := GetDeployments(clientset, ctx, namespace)

	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if item.Spec.Template.ObjectMeta.Labels[key] == value {
			//if item.Spec.Template.ObjectMeta.Labels["app.kubernetes.io/managed-by"] == "Helm" {
			resources = append(resources, item)
		}
	}

	return resources, nil
}
