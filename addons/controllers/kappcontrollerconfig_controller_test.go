// Copyright 2022 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	clusterapiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/tanzu-framework/addons/pkg/constants"
	addontypes "github.com/vmware-tanzu/tanzu-framework/addons/pkg/types"
	"github.com/vmware-tanzu/tanzu-framework/addons/pkg/util"
	"github.com/vmware-tanzu/tanzu-framework/addons/testutil"
	runv1alpha3 "github.com/vmware-tanzu/tanzu-framework/apis/run/v1alpha3"
)

var _ = Describe("KappControllerConfig Reconciler", func() {
	var (
		kappConfigCRName        string
		clusterResourceFilePath string
	)

	JustBeforeEach(func() {
		By("Creating a cluster and a KappControllerConfig")
		f, err := os.Open(clusterResourceFilePath)
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()
		err = testutil.CreateResources(f, cfg, dynamicClient)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		By("Deleting cluster and KappControllerConfig")
		f, err := os.Open(clusterResourceFilePath)
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()
		// Best effort resource deletion
		_ = testutil.DeleteResources(f, cfg, dynamicClient, true)
	})

	Context("reconcile KappControllerConfig for management cluster", func() {

		BeforeEach(func() {
			kappConfigCRName = "test-kapp-controller-1"
			clusterResourceFilePath = "testdata/test-kapp-controller-1.yaml"
		})

		It("Should reconcile KappControllerConfig and create data value secret on management cluster", func() {

			key := client.ObjectKey{
				Namespace: "default",
				Name:      kappConfigCRName,
			}

			cluster := &clusterapiv1beta1.Cluster{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, key, cluster)
				return err == nil
			}, waitTimeout, pollingInterval).Should(BeTrue())

			Eventually(func() bool {
				config := &runv1alpha3.KappControllerConfig{}
				err := k8sClient.Get(ctx, key, config)
				if err != nil {
					return false
				}
				// check owner reference
				if len(config.OwnerReferences) == 0 {
					return false
				}
				Expect(len(config.OwnerReferences)).Should(Equal(1))
				Expect(config.OwnerReferences[0].Name).Should(Equal(kappConfigCRName))

				// check spec values
				Expect(config.Spec.Namespace).Should(Equal("test-ns"))
				Expect(config.Spec.KappController.CreateNamespace).Should(Equal(true))
				Expect(config.Spec.KappController.GlobalNamespace).Should(Equal("tanzu-package-repo-global"))
				Expect(config.Spec.KappController.Deployment.Concurrency).Should(Equal(4))
				Expect(config.Spec.KappController.Deployment.HostNetwork).Should(Equal(true))
				Expect(config.Spec.KappController.Deployment.PriorityClassName).Should(Equal("system-cluster-critical"))
				Expect(config.Spec.KappController.Deployment.APIPort).Should(Equal(10100))
				Expect(config.Spec.KappController.Deployment.MetricsBindAddress).Should(Equal("0"))
				Expect(config.Spec.KappController.Deployment.Tolerations).ShouldNot(BeNil())

				return true
			}, waitTimeout, pollingInterval).Should(BeTrue())

			Eventually(func() bool {
				secretKey := client.ObjectKey{
					Namespace: "default",
					Name:      util.GenerateDataValueSecretName(kappConfigCRName, constants.KappControllerAddonName),
				}
				secret := &v1.Secret{}
				err := k8sClient.Get(ctx, secretKey, secret)
				if err != nil {
					return false
				}
				Expect(secret.Type).Should(Equal(v1.SecretTypeOpaque))

				// check data value secret contents
				secretData := string(secret.Data["values.yaml"])
				Expect(strings.Contains(secretData, "createNamespace: true")).Should(BeTrue())
				Expect(strings.Contains(secretData, "globalNamespace: tanzu-package-repo-global")).Should(BeTrue())
				Expect(strings.Contains(secretData, "concurrency: 4")).Should(BeTrue())
				Expect(strings.Contains(secretData, "hostNetwork: true")).Should(BeTrue())
				Expect(strings.Contains(secretData, "coreDNSIP: 100.64.0.10")).Should(BeTrue())
				Expect(strings.Contains(secretData, "- key: CriticalAddonsOnly")).Should(BeTrue())
				Expect(strings.Contains(secretData, "key: node-role.kubernetes.io/master")).Should(BeTrue())
				Expect(strings.Contains(secretData, "key: node.kubernetes.io/not-ready")).Should(BeTrue())
				Expect(strings.Contains(secretData, "key: node.cloudprovider.kubernetes.io/uninitialized")).Should(BeTrue())
				Expect(strings.Contains(secretData, "apiPort: 10100")).Should(BeTrue())
				Expect(strings.Contains(secretData, "metricsBindAddress: \"0\"")).Should(BeTrue())

				if !strings.Contains(secretData, "caCerts: dummyCertificate") ||
					!strings.Contains(secretData, "httpsProxy: bar.com") ||
					!strings.Contains(secretData, "noProxy: foobar.com") {
					return false
				}

				// user input should override cluster-wide config
				if !strings.Contains(secretData, "httpProxy: overwrite.foo.com") {
					return false
				}

				return true
			}, waitTimeout, pollingInterval).Should(BeTrue())

			Eventually(func() bool {
				// Check status.secretRef after reconciliation
				config := &runv1alpha3.KappControllerConfig{}
				err := k8sClient.Get(ctx, key, config)
				if err != nil {
					return false
				}
				Expect(config.Status.SecretRef).Should(Equal(util.GenerateDataValueSecretName(kappConfigCRName, constants.KappControllerAddonName)))
				return true
			}, waitTimeout, pollingInterval).Should(BeTrue())

		})

	})

	Context("reconcile KappControllerConfig when it's paused", func() {

		BeforeEach(func() {
			kappConfigCRName = "test-kapp-controller-2"
			clusterResourceFilePath = "testdata/test-kapp-controller-2.yaml"
		})

		It("Should not reconcile KappControllerConfig if it's paused", func() {

			key := client.ObjectKey{
				Namespace: "default",
				Name:      kappConfigCRName,
			}

			cluster := &clusterapiv1beta1.Cluster{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, key, cluster)
				return err == nil
			}, waitTimeout, pollingInterval).Should(BeTrue())

			Eventually(func() bool {
				config := &runv1alpha3.KappControllerConfig{}
				err := k8sClient.Get(ctx, key, config)
				if err != nil {
					return false
				}
				// check pause annotation
				Expect(config.Annotations).ToNot(BeNil())
				_, ok := config.Annotations[addontypes.AddonPausedAnnotation]
				Expect(ok).Should(BeTrue())
				return true
			}, waitTimeout, pollingInterval).Should(BeTrue())

			// wait 15 seconds before checking if the data values secret is created or not
			time.Sleep(15 * time.Second)
			Eventually(func() bool {
				secretKey := client.ObjectKey{
					Namespace: "default",
					Name:      util.GenerateDataValueSecretName(kappConfigCRName, constants.KappControllerAddonName),
				}
				secret := &v1.Secret{}
				err := k8sClient.Get(ctx, secretKey, secret)
				if err != nil {
					// retry if error is not IsNotFound
					return errors.IsNotFound(err)
				}
				// error should have happened
				Expect(err).ShouldNot(BeNil())
				return false
			}, waitTimeout, pollingInterval).Should(BeTrue())

		})

	})

})
