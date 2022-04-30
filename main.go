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
		fmt.Println("\tapproach for resource type virtualservices : get-all | set-weights | new-name")
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
		} else if value == "set-weights" {
			fmt.Println("using set-weights")
			setVirtualServiceWeights(dynamic, ctx, "python-api", namespace, 50, 50)
		} else if value == "new-name" {
			fmt.Println("using new-name")
			setVirtualServiceNewName(dynamic, ctx, "python-api", "python-api-v1", namespace)

		} else {
			fmt.Println("unknown approach")
		}
	}
}

func getVirtualServices(dynamic dynamic.Interface, ctx context.Context,
	namespace string) ([]unstructured.Unstructured, error) {
	fmt.Println("using dynamic")
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
			fmt.Println("==========================")
			gw := spec.(map[string]interface{})["gateways"].(interface{})
			fmt.Printf("gateway %s\n", gw)

			hosts := spec.(map[string]interface{})["hosts"].(interface{})
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
			port_map := destination["port"].(interface{})
			fmt.Printf("port_map%s\n", port_map)
			port := port_map.(map[string]interface{})["number"].(int64)
			fmt.Printf("port %d\n", port)

			fmt.Printf("Name %s\n", item.GetName())
			// fmt.Printf("Namespace %s\n", item.GetNamespace())
			// fmt.Printf("Kind %s\n", item.GetKind())
			// fmt.Printf("Labels %s\n", item.GetLabels())
			// fmt.Printf("APIVersion %s\n", item.GetAPIVersion())
			// fmt.Printf("UID %s\n", item.GetUID())
			// fmt.Printf("ResourceVersion %s\n", item.GetResourceVersion())
			// fmt.Printf("Annotations %s\n", item.GetAnnotations())
			// fmt.Printf("%+v\n", item)
		}
	}

	return items, nil
}

func setVirtualServiceNewName(client dynamic.Interface, ctx context.Context,
// in this particular example only one virtualservice is returned, thus
// the hardcoding of the index is fine for now
	virtualServiceName, newName, namespace string) error {

	items, err := getVirtualServices(client, ctx, namespace)
	if err != nil {
		fmt.Println("error in getting virtualservices:", err)
	}

	//  Create a GVR which represents an Istio Virtual Service.
	virtualServiceGVR := schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "virtualservices",
	}

	virtualservice := &unstructured.Unstructured{
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
					"hosts": []string{"python-api-dev.preview.graingercloud.com"},
					// "hosts": []string{},
					"gateways": []string{items[0].GetName()},
					"http": []map[string]interface{}{
						{
							"route": []map[string]interface{}{
								{
									"destination": map[string]interface{}{
										// "host": "python-api-v1",
										"host": newName,
										"port": 80,
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
	// result, err := client.Resource(virtualServiceGVR).Namespace(namespace).Update(ctx, virtualservice, metav1.UpdateOptions{DryRun: []string{"All"}})
	result, err := client.Resource(virtualServiceGVR).Namespace(namespace).Update(ctx, virtualservice, metav1.UpdateOptions{})

	if err != nil {
		fmt.Println("error in updating virtualservice:", err)
		return err
	}

	fmt.Println("VirtualService update result: ", result)

	return nil
}

//  patchStringValue specifies a patch operation for a string.
// type patchStringValue struct {
// 	Op    string `json:"op"`
// 	Path  string `json:"path"`
// 	Value string `json:"value"`
// }

//  patchStringValue specifies a patch operation for a uint32
// type patchUInt32Value struct {
// 	Op    string `json:"op"`
// 	Path  string `json:"path"`
// 	Value uint32 `json:"value"`
// }

// https://gist.github.com/dwmkerr/7332888e092156ce8ce4ea551b0c321f
func setVirtualServiceWeights(client dynamic.Interface, ctx context.Context, virtualServiceName, namespace string, weight1, weight2 uint32) error {
	//  Create a GVR which represents an Istio Virtual Service.
	virtualServiceGVR := schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "virtualservices",
	}

	//  PATCH - begin
	//  Weight the two routes - 50/50.
	// patchPayload := make([]patchUInt32Value, 2)
	// patchPayload[0].Op = "replace"
	// patchPayload[0].Path = "/spec/http/0/route/0/weight"
	// patchPayload[0].Value = weight1
	// patchPayload[1].Op = "replace"
	// patchPayload[1].Path = "/spec/http/0/route/1/weight"
	// patchPayload[1].Value = weight2
	// patchBytes, _ := json.Marshal(patchPayload)

	// fmt.Println("before patchBytes: ", string(patchBytes))

	// https://github.com/kubernetes/client-go/blob/master/dynamic/interface.go
	//  Apply the patch to the 'service2' service. - patch won't work cause that is an update of an existing specific structure/state
	// result, err := client.Resource(virtualServiceGVR).Namespace(namespace).Patch(ctx, virtualServiceName, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
	//  PATCH - end

	virtualservice := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.istio.io/v1alpha3",
			"kind":       "VirtualService",
			"metadata": map[string]interface{}{
				"name":            "python-api",
				"namespace":       "resiliency-dev",
				"uid":             "943b8484-3d5b-477b-b5cb-af09a2c3daee",
				"resourceVersion": "1553172",
			},
			"spec": &unstructured.UnstructuredList{
				Object: map[string]interface{}{
					"hosts": []string{"python-api-dev.preview.graingercloud.com"},
					"http": []map[string]interface{}{
						{
							"route": []map[string]interface{}{
								{
									"destination": map[string]interface{}{
										"host": "python-api",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Update(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error)
	// dry run
	result, err := client.Resource(virtualServiceGVR).Namespace(namespace).Update(ctx, virtualservice, metav1.UpdateOptions{DryRun: []string{"All"}})
	// result, err := client.Resource(virtualServiceGVR).Namespace(namespace).Update(ctx, virtualservice, metav1.UpdateOptions{})

	if err != nil {
		fmt.Println("error in patching/updating/etc virtualservice:", err)
		return err
	}

	fmt.Println("VirtualService patch/update/etc result: ", result)

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
