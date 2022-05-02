package main

// https://itnext.io/generically-working-with-kubernetes-resources-in-go-53bce678f887

import (
	"context"
	"fmt"
	"os"

	"github.com/itchyny/gojq"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

func main() {
	args := os.Args
	if len(args) != 3 {
		fmt.Println("Usage: go run ./main.go <resource> <approach>")
		fmt.Println("\tresource: deployments | virtualservices")
		fmt.Println("\tapproach for resource type deployments: clientset | dynamic | dynamic-jq | clientset-query")
		fmt.Println("\tapproach for resource type virtualservices : get-all | fault | reverse-fault")
		return
	}

	ctx := context.Background()
	config := ctrl.GetConfigOrDie()
	clientset := kubernetes.NewForConfigOrDie(config)
	dynamic := dynamic.NewForConfigOrDie(config)

	namespace := "resiliency-dev"

	if args[1] == "deployments" {
		fmt.Println("using deployments")
		getDeployments(args, clientset, dynamic, ctx, namespace)
	} else if args[1] == "virtualservices" {
		fmt.Println("using virtualservices")
		value := args[2]
		if value == "get-all" {
			fmt.Println("using get-all")
			getVirtualServices(dynamic, ctx, namespace)
		} else if value == "fault" {
			fmt.Println("using fault")
			setVirtualServiceFaultForApp(dynamic, ctx, "app.kubernetes.io/name", "python-api", namespace)
		} else if value == "reverse-fault" {
			fmt.Println("using reverse-fault")
			removeVirtualServiceFaultForApp(dynamic, ctx, "app.kubernetes.io/name", "python-api", namespace)
		} else {
			fmt.Println("unknown approach")
		}
	}
}

func getVirtualServices(dynamic dynamic.Interface, ctx context.Context,
	namespace string) ([]unstructured.Unstructured, error) {
	fmt.Println("using dynamic getAllVirtualServices")
	// get all virtualservices
	resourceId := schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "virtualservices",
	}

	items, err := GetResourcesDynamically(dynamic, ctx, namespace, resourceId)
	if err != nil {
		fmt.Println(err)
		return nil, err
	} else {
		for _, item := range items {
			spec := item.Object["spec"]

			fmt.Println(spec)
			for k, v := range spec.(map[string]interface{}) {
				fmt.Println(k, v)
			}
		}
	}

	return items, nil
}

func getVirtualServiceByAppName(dynamic dynamic.Interface, ctx context.Context, labelKey,
	labelAppNameValue, namespace string) ([]unstructured.Unstructured, error) {
	fmt.Println("using dynamic getVirtualServiceByAppName")

	resourceId := schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "virtualservices",
	}

	query := fmt.Sprintf(".metadata.labels[\"%s\"] == \"%s\"", labelKey, labelAppNameValue)

	items, err := GetResourcesDynamicallyByJq(dynamic, ctx, namespace, resourceId, query)
	if len(items) == 0 {
		return nil, fmt.Errorf("no virtualservices found for app %s, in namespace %s", labelAppNameValue, namespace)
	}

	if len(items) > 1 {
		return nil, fmt.Errorf("more than one virtualservice found for app %s, in namespace %s, found total of %d", labelAppNameValue, namespace, len(items))
	}

	if err != nil {
		fmt.Println("error in getVirtualServiceByName", err)
		return nil, err
	} else {
		for _, item := range items {
			spec := item.Object["spec"]

			// fmt.Println(spec)

			// fmt.Println(spec)
			// for k, v := range spec.(map[string]interface{}) {
			// 	fmt.Println(k, v)
			// }

			fmt.Println("==========================")
			gateway := spec.(map[string]interface{})["gateways"]
			fmt.Printf("gateway %s\n", gateway)

			hosts := spec.(map[string]interface{})["hosts"]
			fmt.Printf("hosts %s\n", hosts)

			http := spec.(map[string]interface{})["http"].([]interface{})
			http_0 := http[0].(map[string]interface{})
			fmt.Printf("http %s\n", http[0])

			route := http_0["route"].([]interface{})
			route_0 := route[0].(map[string]interface{})
			fmt.Printf("route %s\n", route[0])

			destination := route_0["destination"].(map[string]interface{})
			fmt.Printf("destination %s\n", destination)

			host := destination["host"].(string)
			fmt.Printf("host %s\n", host)

			port_map := destination["port"]
			fmt.Printf("port_map %s\n", port_map)

			port := port_map.(map[string]interface{})["number"].(int)
			fmt.Printf("port %d\n", port)

			fmt.Printf("Name %s\n", item.GetName())
			fmt.Printf("Namespace %s\n", item.GetNamespace())
			fmt.Printf("Kind %s\n", item.GetKind())
			fmt.Printf("Labels %s\n", item.GetLabels())
			fmt.Printf("APIVersion %s\n", item.GetAPIVersion())
			fmt.Printf("UID %s\n", item.GetUID())
			fmt.Printf("ResourceVersion %s\n", item.GetResourceVersion())
			fmt.Printf("Annotations %s\n", item.GetAnnotations())
			fmt.Printf("%+v\n", item)
		}
	}

	return items, nil
}

func setVirtualServiceFaultForApp(client dynamic.Interface, ctx context.Context,
	labelKey, labelAppNameValue, namespace string) error {

	items, err := getVirtualServiceByAppName(client, ctx, labelKey, labelAppNameValue, namespace)
	if err != nil {
		fmt.Println("error in getting virtualservices:", err)
		return err
	}

	spec := items[0].Object["spec"]

	gateway := spec.(map[string]interface{})["gateways"]
	fmt.Printf("gateway %s\n", gateway)

	hosts := spec.(map[string]interface{})["hosts"]
	fmt.Printf("hosts %s\n", hosts)

	http := spec.(map[string]interface{})["http"].([]interface{})
	http_0 := http[0].(map[string]interface{})
	fmt.Printf("http %s\n", http[0])

	route := http_0["route"].([]interface{})
	route_0 := route[0].(map[string]interface{})
	fmt.Printf("route %s\n", route[0])

	destination := route_0["destination"].(map[string]interface{})
	fmt.Printf("destination %s\n", destination)

	host := destination["host"].(string)
	fmt.Printf("host %s\n", host)

	port_map := destination["port"]
	fmt.Printf("port_map%s\n", port_map)

	port := port_map.(map[string]interface{})["number"]
	fmt.Printf("port %d\n", port)

	//  Create a GVR which represents an Istio Virtual Service.
	virtualServiceGVR := schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "virtualservices",
	}

	// virtualserviceNoFault := &unstructured.Unstructured{
	// 	Object: map[string]interface{}{
	// 		"apiVersion": "networking.istio.io/v1alpha3",
	// 		"kind":       "VirtualService",
	// 		"metadata": map[string]interface{}{
	// 			"name":            items[0].GetName(),
	// 			"namespace":       items[0].GetNamespace(),
	// 			"uid":             items[0].GetUID(),
	// 			"resourceVersion": items[0].GetResourceVersion(),
	// 			"annotations":     items[0].GetAnnotations(),
	// 			"labels":          items[0].GetLabels(),
	// 		},
	// 		"spec": &unstructured.UnstructuredList{
	// 			Object: map[string]interface{}{
	// 				"hosts":    hosts,
	// 				"gateways": gateway,
	// 				"http": []map[string]interface{}{
	// 					{
	// 						"route": []map[string]interface{}{
	// 							{
	// 								"destination": map[string]interface{}{
	// 									"host": host,
	// 									"port": map[string]interface{}{
	// 										"number": port,
	// 									},
	// 								},
	// 							},
	// 						},
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// }

	virtualserviceFault := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.istio.io/v1alpha3",
			"kind":       "VirtualService",
			"metadata": map[string]interface{}{
				"name":            items[0].GetName(),
				"namespace":       items[0].GetNamespace(),
				"uid":             items[0].GetUID(),
				"resourceVersion": items[0].GetResourceVersion(),
				"annotations":     items[0].GetAnnotations(),
				"labels":          items[0].GetLabels(),
			},
			"spec": &unstructured.UnstructuredList{
				Object: map[string]interface{}{
					"hosts":    hosts,
					"gateways": gateway,
					"http": []map[string]interface{}{
						{
							"fault": map[string]interface{}{
								"abort": map[string]interface{}{
									"httpStatus": 500,
									"percentage": map[string]interface{}{
										"value": 100,
									},
								},
							},
							"route": []map[string]interface{}{
								{
									"destination": map[string]interface{}{
										"host": host,
										"port": map[string]interface{}{
											"number": port,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// dry run
	// result, err := client.Resource(virtualServiceGVR).Namespace(namespace).Update(ctx, virtualserviceFault, metav1.UpdateOptions{DryRun: []string{"All"}})
	result, err := client.Resource(virtualServiceGVR).Namespace(namespace).Update(ctx, virtualserviceFault, metav1.UpdateOptions{})

	if err != nil {
		fmt.Println("error in updating virtualservice:", err)
		return err
	}

	fmt.Println("VirtualService update result: ", result)

	return nil
}

func removeVirtualServiceFaultForApp(client dynamic.Interface, ctx context.Context,
	labelKey, labelAppNameValue, namespace string) error {

	items, err := getVirtualServiceByAppName(client, ctx, labelKey, labelAppNameValue, namespace)
	if err != nil {
		fmt.Println("error in getting virtualservices:", err)
		return err
	}

	spec := items[0].Object["spec"]

	gateway := spec.(map[string]interface{})["gateways"]
	fmt.Printf("gateway %s\n", gateway)

	hosts := spec.(map[string]interface{})["hosts"]
	fmt.Printf("hosts %s\n", hosts)

	http := spec.(map[string]interface{})["http"].([]interface{})
	http_0 := http[0].(map[string]interface{})
	fmt.Printf("http %s\n", http[0])

	route := http_0["route"].([]interface{})
	route_0 := route[0].(map[string]interface{})
	fmt.Printf("route %s\n", route[0])

	destination := route_0["destination"].(map[string]interface{})
	fmt.Printf("destination %s\n", destination)

	host := destination["host"].(string)
	fmt.Printf("host %s\n", host)

	port_map := destination["port"]
	fmt.Printf("port_map%s\n", port_map)

	port := port_map.(map[string]interface{})["number"]
	fmt.Printf("port %d\n", port)

	//  Create a GVR which represents an Istio Virtual Service.
	virtualServiceGVR := schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "virtualservices",
	}

	virtualserviceNoFault := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.istio.io/v1alpha3",
			"kind":       "VirtualService",
			"metadata": map[string]interface{}{
				"name":            items[0].GetName(),
				"namespace":       items[0].GetNamespace(),
				"uid":             items[0].GetUID(),
				"resourceVersion": items[0].GetResourceVersion(),
				"annotations":     items[0].GetAnnotations(),
				"labels":          items[0].GetLabels(),
			},
			"spec": &unstructured.UnstructuredList{
				Object: map[string]interface{}{
					"hosts":    hosts,
					"gateways": gateway,
					"http": []map[string]interface{}{
						{
							"route": []map[string]interface{}{
								{
									"destination": map[string]interface{}{
										"host": host,
										"port": map[string]interface{}{
											"number": port,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// dry run
	// result, err := client.Resource(virtualServiceGVR).Namespace(namespace).Update(ctx, virtualserviceNoFault, metav1.UpdateOptions{DryRun: []string{"All"}})
	result, err := client.Resource(virtualServiceGVR).Namespace(namespace).Update(ctx, virtualserviceNoFault, metav1.UpdateOptions{})

	if err != nil {
		fmt.Println("error in updating virtualservice:", err)
		return err
	}

	fmt.Println("VirtualService update result: ", result)

	return nil
}

func getDeployments(args []string, clientset *kubernetes.Clientset, dynamic dynamic.Interface, ctx context.Context, namespace string) {
	value := args[2]
	if value == "clientset" {
		fmt.Println("using client set")
		items, err := GetDeployments(clientset, ctx, namespace)

		if err != nil {
			fmt.Println(err)
		} else {
			for _, item := range items {
				fmt.Printf("%+v\n", item)
			}
		}
	} else if value == "dynamic" {
		fmt.Println("using dynamic")
		resourceId := schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		}

		items, err := GetResourcesDynamically(dynamic, ctx, namespace, resourceId)

		if err != nil {
			fmt.Println(err)
		} else {
			for _, item := range items {
				fmt.Printf("%+v\n", item)
			}
		}
	} else if value == "dynamic-jq" {
		fmt.Println("using dynamic-jq")
		resourceId := schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		}
		query := ".metadata.labels[\"app.kubernetes.io/managed-by\"] == \"Helm\""
		// query := ".metadata.labels[\"app.kubernetes.io/managed-by\"] == \"HelmABC\"" // purposefully wrong
		// if query is not found, nothing is returned and no error is thrown, nothing prints
		items, err := GetResourcesDynamicallyByJq(dynamic, ctx, namespace, resourceId, query)
		if err != nil {
			fmt.Println(err)
		} else {
			for _, item := range items {
				fmt.Printf("%+v\n", item)
			}
		}
	} else if value == "clientset-query" {
		fmt.Println("using clientset-query")
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
		fmt.Println("Usage: go run ./main.go <resource> <approach>")
		fmt.Println("\tresource: deployments")
		fmt.Println("\tapproach for resource type deployments: clientset | dynamic | dynamic-jq | clientset-query")
		fmt.Println("\targs passed in:", args[1:])
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
	namespace string, resourceId schema.GroupVersionResource) (
	[]unstructured.Unstructured, error) {

	list, err := dynamic.Resource(resourceId).Namespace(namespace).
		List(ctx, metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func GetResourcesDynamicallyByJq(dynamic dynamic.Interface, ctx context.Context,
	namespace string, resoureId schema.GroupVersionResource, jq string) (
	[]unstructured.Unstructured, error) {
	fmt.Println("GetResourcesDynamicallyByJq query is:", jq)
	resources := make([]unstructured.Unstructured, 0)

	query, err := gojq.Parse(jq)
	if err != nil {
		return nil, err
	}

	items, err := GetResourcesDynamically(dynamic, ctx, namespace, resoureId)
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
