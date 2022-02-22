package webhooks

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cacheddiscovery "k8s.io/client-go/discovery/cached/memory"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	fake "k8s.io/client-go/kubernetes/fake"
	ctrl "sigs.k8s.io/controller-runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/vmware-tanzu/tanzu-framework/addons/test/builder"
	runv1alpha3 "github.com/vmware-tanzu/tanzu-framework/apis/run/v1alpha3"
)

var (
	ctx        = ctrl.SetupSignalHandler()
	fakeScheme = runtime.NewScheme()
)

func init() {
	_ = runv1alpha3.AddToScheme(fakeScheme)
}

func TestClusterBootstrapCreationValidation(t *testing.T) {
	g := NewWithT(t)

	namespace := "default"

	in := builder.ClusterBootstrap(namespace, "class1").
		WithCNIPackage(builder.ClusterBootstrapPackage("cni.example.com.1.17.2").WithProviderRef("run.tanzu.vmware.com", "foo", "bar").Build()).
		WithAdditionalPackage(builder.ClusterBootstrapPackage("pinniped.example.com.1.11.3").Build()).
		Build()

	fakeClient := fakeclient.NewClientBuilder().
		WithScheme(fakeScheme).
		Build()

	fakeDynamicClient := fakedynamic.NewSimpleDynamicClient(fakeScheme)
	clientSet := fake.NewSimpleClientset()
	fakeDiscovery, ok := clientSet.Discovery().(*fakediscovery.FakeDiscovery)
	g.Expect(ok).To(BeTrue())
	fakeDiscovery.Fake.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "packages.operators.coreos.com/v1",
			APIResources: []metav1.APIResource{
				{
					Kind: "PackageManifest",
				},
			},
		},
	}

	// Create the webhook and add the fakeClient as its client.
	webhook := &ClusterBootstrap{
		Client:                fakeClient,
		DynamicClient:         fakeDynamicClient,
		CachedDiscoveryClient: cacheddiscovery.NewMemCacheClient(clientSet.Discovery()),
	}
	g.Expect(webhook.ValidateCreate(ctx, in)).ToNot(HaveOccurred())

}
