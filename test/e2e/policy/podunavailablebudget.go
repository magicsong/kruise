/*
Copyright 2021 The Kruise Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package policy

import (
	"context"
	"fmt"
	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilpointer "k8s.io/utils/pointer"
	"time"

	policyv1alpha1 "github.com/openkruise/kruise/apis/policy/v1alpha1"
	kruiseclientset "github.com/openkruise/kruise/pkg/client/clientset/versioned"
	"github.com/openkruise/kruise/test/e2e/framework"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

var _ = SIGDescribe("PodUnavailableBudget", func() {
	f := framework.NewDefaultFramework("podunavailablebudget")
	var ns string
	var c clientset.Interface
	var kc kruiseclientset.Interface
	var tester *framework.PodUnavailableBudgetTester
	var sidecarTester *framework.SidecarSetTester

	ginkgo.BeforeEach(func() {
		c = f.ClientSet
		kc = f.KruiseClientSet
		ns = f.Namespace.Name
		tester = framework.NewPodUnavailableBudgetTester(c, kc)
		sidecarTester = framework.NewSidecarSetTester(c, kc)
	})

	framework.KruiseDescribe("podUnavailableBudget functionality [podUnavailableBudget]", func() {

		ginkgo.AfterEach(func() {
			if ginkgo.CurrentGinkgoTestDescription().Failed {
				framework.DumpDebugInfo(c, ns)
			}
			framework.Logf("Deleting all PodUnavailableBudgets and Deployments in cluster")
			tester.DeletePubs(ns)
			tester.DeleteDeployments(ns)
			tester.DeleteCloneSets(ns)
			sidecarTester.DeleteSidecarSets()
		})

		ginkgo.It("PodUnavailableBudget selector no matched pods", func() {
			// create pub
			pub := tester.NewBasePub(ns)
			ginkgo.By(fmt.Sprintf("Creating PodUnavailableBudget(%s.%s)", pub.Namespace, pub.Name))
			tester.CreatePub(pub)

			// create deployment
			deployment := tester.NewBaseDeployment(ns)
			deployment.Spec.Selector.MatchLabels["pub-controller"] = "false"
			deployment.Spec.Template.Labels["pub-controller"] = "false"
			ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deployment.Namespace, deployment.Name))
			tester.CreateDeployment(deployment)

			// wait 10 seconds
			time.Sleep(time.Second * 10)
			ginkgo.By(fmt.Sprintf("check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			expectStatus := &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 0,
				DesiredAvailable:   0,
				CurrentAvailable:   0,
				TotalReplicas:      0,
			}
			setPubStatus(expectStatus)
			pub, err := kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus := &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))
			ginkgo.By("PodUnavailableBudget selector no matched pods done")
		})

		ginkgo.It("PodUnavailableBudget selector pods and delete deployment ignore", func() {
			// create pub
			pub := tester.NewBasePub(ns)
			ginkgo.By(fmt.Sprintf("Creating PodUnavailableBudget(%s.%s)", pub.Namespace, pub.Name))
			tester.CreatePub(pub)

			// create deployment
			deployment := tester.NewBaseDeployment(ns)
			ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deployment.Namespace, deployment.Name))
			tester.CreateDeployment(deployment)

			// wait 10 seconds
			time.Sleep(time.Second * 10)
			ginkgo.By(fmt.Sprintf("check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			expectStatus := &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 1,
				DesiredAvailable:   1,
				CurrentAvailable:   2,
				TotalReplicas:      2,
			}
			setPubStatus(expectStatus)
			pub, err := kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus := pub.Status.DeepCopy()
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			// delete deployment
			ginkgo.By(fmt.Sprintf("Deleting Deployment(%s.%s)", deployment.Namespace, deployment.Name))
			err = c.AppsV1().Deployments(deployment.Namespace).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// wait 30 seconds
			time.Sleep(time.Second * 10)
			ginkgo.By(fmt.Sprintf("waiting 10 seconds, and check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			expectStatus = &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 0,
				DesiredAvailable:   0,
				CurrentAvailable:   0,
				TotalReplicas:      0,
			}
			setPubStatus(expectStatus)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus = pub.Status.DeepCopy()
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))
			pods, err := sidecarTester.GetSelectorPods(deployment.Namespace, deployment.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pods).To(gomega.HaveLen(0))

			ginkgo.By("PodUnavailableBudget selector pods and delete deployment reject done")
		})

		ginkgo.It("PodUnavailableBudget targetReference pods, update failed image and block", func() {
			// create pub
			pub := tester.NewBasePub(ns)
			pub.Spec.Selector = nil
			pub.Spec.TargetReference = &policyv1alpha1.TargetReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "busybox",
			}
			ginkgo.By(fmt.Sprintf("Creating PodUnavailableBudget(%s.%s)", pub.Namespace, pub.Name))
			tester.CreatePub(pub)

			// create deployment
			deployment := tester.NewBaseDeployment(ns)
			ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deployment.Namespace, deployment.Name))
			tester.CreateDeployment(deployment)

			// wait 10 seconds
			time.Sleep(time.Second * 10)
			ginkgo.By(fmt.Sprintf("check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			expectStatus := &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 1,
				DesiredAvailable:   1,
				CurrentAvailable:   2,
				TotalReplicas:      2,
			}
			setPubStatus(expectStatus)
			pub, err := kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus := &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			// update failed image
			ginkgo.By(fmt.Sprintf("update Deployment(%s.%s) failed image(busybox:failed)", deployment.Namespace, deployment.Name))
			deployment.Spec.Template.Spec.Containers[0].Image = "busybox:failed"
			_, err = c.AppsV1().Deployments(deployment.Namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			//wait 20 seconds
			ginkgo.By(fmt.Sprintf("waiting 20 seconds, and check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			time.Sleep(time.Second * 20)
			expectStatus = &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 0,
				DesiredAvailable:   1,
				CurrentAvailable:   1,
				TotalReplicas:      2,
			}
			setPubStatus(expectStatus)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus = &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			// check now pod
			pods, err := sidecarTester.GetSelectorPods(deployment.Namespace, deployment.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			noUpdatePods := make([]corev1.Pod, 0)
			for _, pod := range pods {
				if pod.Spec.Containers[0].Image == "busybox:failed" || !pod.DeletionTimestamp.IsZero() {
					continue
				}
				noUpdatePods = append(noUpdatePods, *pod)
			}
			gomega.Expect(noUpdatePods).To(gomega.HaveLen(1))

			// update success image
			ginkgo.By(fmt.Sprintf("update Deployment(%s.%s) success image(busybox:1.33)", deployment.Namespace, deployment.Name))
			deployment.Spec.Template.Spec.Containers[0].Image = "busybox:1.33"
			_, err = c.AppsV1().Deployments(deployment.Namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			//wait 20 seconds
			ginkgo.By(fmt.Sprintf("waiting 20 seconds, and check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			time.Sleep(time.Second * 20)
			expectStatus = &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 1,
				DesiredAvailable:   1,
				CurrentAvailable:   2,
				TotalReplicas:      2,
			}
			setPubStatus(expectStatus)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus = &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			//check pods
			pods, err = sidecarTester.GetSelectorPods(deployment.Namespace, deployment.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			newPods := make([]corev1.Pod, 0)
			for _, pod := range pods {
				if !pod.DeletionTimestamp.IsZero() || pod.Spec.Containers[0].Image != "busybox:1.33" {
					continue
				}
				newPods = append(newPods, *pod)
			}
			gomega.Expect(newPods).To(gomega.HaveLen(2))
			ginkgo.By("PodUnavailableBudget targetReference pods, update failed image and block done")
		})

		ginkgo.It("PodUnavailableBudget selector two deployments, deployment.strategy.maxUnavailable=100%, pub.spec.maxUnavailable=20%, and update success image", func() {
			// create pub
			pub := tester.NewBasePub(ns)
			pub.Spec.MaxUnavailable = &intstr.IntOrString{
				Type:   intstr.String,
				StrVal: "20%",
			}
			ginkgo.By(fmt.Sprintf("Creating PodUnavailableBudget(%s.%s)", pub.Namespace, pub.Name))
			tester.CreatePub(pub)

			// create deployment1
			deployment := tester.NewBaseDeployment(ns)
			deployment.Spec.Replicas = utilpointer.Int32Ptr(5)
			deploymentIn1 := deployment.DeepCopy()
			deploymentIn1.Name = fmt.Sprintf("%s-1", deploymentIn1.Name)
			ginkgo.By(fmt.Sprintf("Creating Deployment1(%s.%s)", deploymentIn1.Namespace, deploymentIn1.Name))
			tester.CreateDeployment(deploymentIn1)
			// create deployment2
			deploymentIn2 := deployment.DeepCopy()
			deploymentIn2.Name = fmt.Sprintf("%s-2", deploymentIn1.Name)
			ginkgo.By(fmt.Sprintf("Creating Deployment2(%s.%s)", deploymentIn2.Namespace, deploymentIn2.Name))
			tester.CreateDeployment(deploymentIn2)

			// wait 1 seconds
			ginkgo.By(fmt.Sprintf("wait 1 seconds, check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			time.Sleep(time.Second)
			expectStatus := &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 2,
				DesiredAvailable:   8,
				CurrentAvailable:   10,
				TotalReplicas:      10,
			}
			setPubStatus(expectStatus)
			pub, err := kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus := &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			// update success image
			ginkgo.By(fmt.Sprintf("update Deployment-1 and deployment-2 with success image(busybox:1.33)"))
			deploymentIn1.Spec.Template.Spec.Containers[0].Image = "busybox:1.33"
			_, err = c.AppsV1().Deployments(deploymentIn1.Namespace).Update(context.TODO(), deploymentIn1, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			deploymentIn2.Spec.Template.Spec.Containers[0].Image = "busybox:1.33"
			_, err = c.AppsV1().Deployments(deploymentIn2.Namespace).Update(context.TODO(), deploymentIn2, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// wait 1 seconds, and check deployment, pub Status
			ginkgo.By(fmt.Sprintf("wait 1 seconds, and check deployment, pub Status"))
			time.Sleep(time.Second)
			// check deployment
			tester.WaitForDeploymentMinReadyAndRunning([]*apps.Deployment{deploymentIn1, deploymentIn2}, 8)
			// check pods
			pods, err := sidecarTester.GetSelectorPods(deployment.Namespace, deployment.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			newPods := make([]corev1.Pod, 0)
			for _, pod := range pods {
				if !pod.DeletionTimestamp.IsZero() || pod.Spec.Containers[0].Image != "busybox:1.33" {
					continue
				}
				newPods = append(newPods, *pod)
			}
			gomega.Expect(newPods).To(gomega.HaveLen(10))

			time.Sleep(time.Second * 5)
			expectStatus = &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 2,
				DesiredAvailable:   8,
				CurrentAvailable:   10,
				TotalReplicas:      10,
			}
			setPubStatus(expectStatus)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus = &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			ginkgo.By("PodUnavailableBudget selector two deployments, deployment.strategy.maxUnavailable=100%, pub.spec.maxUnavailable=20%, and update success image done")
		})

		ginkgo.It("PodUnavailableBudget selector SidecarSet, inject sidecar container, update failed sidecar image, block", func() {
			// create pub
			pub := tester.NewBasePub(ns)
			ginkgo.By(fmt.Sprintf("Creating PodUnavailableBudget(%s.%s)", pub.Namespace, pub.Name))
			tester.CreatePub(pub)

			// create sidecarset
			sidecarSet := sidecarTester.NewBaseSidecarSet(ns)
			sidecarSet.Spec.Namespace = ns
			sidecarSet.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "busybox",
				},
			}
			sidecarSet.Spec.Containers = []appsv1alpha1.SidecarContainer{
				{
					Container: corev1.Container{
						Name:    "nginx-sidecar",
						Image:   "nginx:1.18",
						Command: []string{"tail", "-f", "/dev/null"},
					},
				},
			}
			sidecarSet.Spec.UpdateStrategy = appsv1alpha1.SidecarSetUpdateStrategy{
				Type: appsv1alpha1.RollingUpdateSidecarSetStrategyType,
				MaxUnavailable: &intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "100%",
				},
			}
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSet.Name))
			sidecarSet = sidecarTester.CreateSidecarSet(sidecarSet)

			// create deployment
			deployment := tester.NewBaseDeployment(ns)
			deployment.Spec.Replicas = utilpointer.Int32Ptr(5)
			ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deployment.Namespace, deployment.Name))
			tester.CreateDeployment(deployment)

			time.Sleep(time.Second)
			// check sidecarSet inject sidecar container
			ginkgo.By(fmt.Sprintf("check sidecarSet inject sidecar container and pub status"))
			pods, err := sidecarTester.GetSelectorPods(deployment.Namespace, deployment.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			pod := pods[0]
			gomega.Expect(pod.Spec.Containers).To(gomega.HaveLen(len(deployment.Spec.Template.Spec.Containers) + len(sidecarSet.Spec.Containers)))

			//check pub status
			time.Sleep(time.Second)
			expectStatus := &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 1,
				DesiredAvailable:   4,
				CurrentAvailable:   5,
				TotalReplicas:      5,
			}
			setPubStatus(expectStatus)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus := &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			// update sidecar container failed image
			ginkgo.By(fmt.Sprintf("update sidecar container failed image(nginx:failed)"))
			sidecarSet, err = kc.AppsV1alpha1().SidecarSets().Get(context.TODO(), sidecarSet.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			sidecarSet.Spec.Containers[0].Image = "nginx:failed"
			sidecarTester.UpdateSidecarSet(sidecarSet)

			// wait 1 seconds, and check sidecarSet upgrade block
			ginkgo.By(fmt.Sprintf("wait 1 seconds, and check sidecarSet upgrade block"))
			time.Sleep(time.Second)
			except := &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      5,
				UpdatedPods:      1,
				UpdatedReadyPods: 0,
				ReadyPods:        4,
			}
			sidecarTester.WaitForSidecarSetMinReadyAndUpgrade(sidecarSet, except, 4)
			time.Sleep(time.Second)
			//check pub status
			expectStatus = &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 0,
				DesiredAvailable:   4,
				CurrentAvailable:   4,
				TotalReplicas:      5,
			}
			setPubStatus(expectStatus)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus = &pub.Status
			// check unavailablePods
			gomega.Expect(nowStatus.UnavailablePods).To(gomega.HaveLen(1))
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			// update sidecar container success image
			ginkgo.By(fmt.Sprintf("update sidecar container success image"))
			sidecarSet, err = kc.AppsV1alpha1().SidecarSets().Get(context.TODO(), sidecarSet.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			sidecarSet.Spec.Containers[0].Image = "nginx:1.19"
			sidecarTester.UpdateSidecarSet(sidecarSet)

			time.Sleep(time.Second)
			// check sidecarSet upgrade success
			ginkgo.By(fmt.Sprintf("check sidecarSet upgrade success"))
			except = &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      5,
				UpdatedPods:      5,
				UpdatedReadyPods: 5,
				ReadyPods:        5,
			}
			sidecarTester.WaitForSidecarSetMinReadyAndUpgrade(sidecarSet, except, 4)
			time.Sleep(time.Second)
			//check pub status
			expectStatus = &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 1,
				DesiredAvailable:   4,
				CurrentAvailable:   5,
				TotalReplicas:      5,
			}
			setPubStatus(expectStatus)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus = &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			ginkgo.By("PodUnavailableBudget selector pods, inject sidecar container, update failed sidecar image, block done")
		})

		ginkgo.It("PodUnavailableBudget selector cloneSet, strategy.type=recreate, update failed image and block", func() {
			// create pub
			pub := tester.NewBasePub(ns)
			ginkgo.By(fmt.Sprintf("Creating PodUnavailableBudget(%s.%s)", pub.Namespace, pub.Name))
			tester.CreatePub(pub)

			// create cloneset
			cloneset := tester.NewBaseCloneSet(ns)
			ginkgo.By(fmt.Sprintf("Creating CloneSet(%s.%s)", cloneset.Namespace, cloneset.Name))
			cloneset = tester.CreateCloneSet(cloneset)

			// wait 10 seconds
			time.Sleep(time.Second * 10)
			ginkgo.By(fmt.Sprintf("check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			expectStatus := &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 1,
				DesiredAvailable:   1,
				CurrentAvailable:   2,
				TotalReplicas:      2,
			}
			setPubStatus(expectStatus)
			pub, err := kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus := &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			// update failed image
			ginkgo.By(fmt.Sprintf("update CloneSet(%s.%s) with failed image(busybox:failed)", cloneset.Namespace, cloneset.Name))
			cloneset.Spec.Template.Spec.Containers[0].Image = "busybox:failed"
			_, err = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Update(context.TODO(), cloneset, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			//wait 20 seconds
			ginkgo.By(fmt.Sprintf("waiting 20 seconds, and check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			time.Sleep(time.Second * 20)
			expectStatus = &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 0,
				DesiredAvailable:   1,
				CurrentAvailable:   1,
				TotalReplicas:      2,
			}
			setPubStatus(expectStatus)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus = &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			// check now pod
			pods, err := sidecarTester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			noUpdatePods := make([]corev1.Pod, 0)
			for _, pod := range pods {
				if pod.Spec.Containers[0].Image == "busybox:failed" || !pod.DeletionTimestamp.IsZero() {
					continue
				}
				noUpdatePods = append(noUpdatePods, *pod)
			}
			gomega.Expect(noUpdatePods).To(gomega.HaveLen(1))

			// update success image
			ginkgo.By(fmt.Sprintf("update CloneSet(%s.%s) success image(busybox:1.33)", cloneset.Namespace, cloneset.Name))
			cloneset, _ = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Get(context.TODO(), cloneset.Name, metav1.GetOptions{})
			cloneset.Spec.Template.Spec.Containers[0].Image = "busybox:1.33"
			cloneset, err = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Update(context.TODO(), cloneset, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			tester.WaitForCloneSetMinReadyAndRunning([]*appsv1alpha1.CloneSet{cloneset}, 1)

			// check pub status
			ginkgo.By(fmt.Sprintf("check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			expectStatus = &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 1,
				DesiredAvailable:   1,
				CurrentAvailable:   2,
				TotalReplicas:      2,
			}
			setPubStatus(expectStatus)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus = &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			//check pods
			pods, err = sidecarTester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			newPods := make([]corev1.Pod, 0)
			for _, pod := range pods {
				if !pod.DeletionTimestamp.IsZero() || pod.Spec.Containers[0].Image != "busybox:1.33" {
					continue
				}
				newPods = append(newPods, *pod)
			}
			gomega.Expect(newPods).To(gomega.HaveLen(2))
			ginkgo.By("PodUnavailableBudget selector cloneSet, update failed image and block done")
		})

		ginkgo.It("PodUnavailableBudget selector cloneSet, strategy.type=in-place, update failed image and block", func() {
			// create pub
			pub := tester.NewBasePub(ns)
			ginkgo.By(fmt.Sprintf("Creating PodUnavailableBudget(%s.%s)", pub.Namespace, pub.Name))
			tester.CreatePub(pub)

			// create cloneset
			cloneset := tester.NewBaseCloneSet(ns)
			cloneset.Spec.UpdateStrategy.Type = appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType
			ginkgo.By(fmt.Sprintf("Creating CloneSet(%s.%s)", cloneset.Namespace, cloneset.Name))
			cloneset = tester.CreateCloneSet(cloneset)

			// wait 10 seconds
			time.Sleep(time.Second * 10)
			ginkgo.By(fmt.Sprintf("check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			expectStatus := &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 1,
				DesiredAvailable:   1,
				CurrentAvailable:   2,
				TotalReplicas:      2,
			}
			setPubStatus(expectStatus)
			pub, err := kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus := &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			// update failed image
			ginkgo.By(fmt.Sprintf("update CloneSet(%s.%s) with failed image(busybox:failed)", cloneset.Namespace, cloneset.Name))
			cloneset.Spec.Template.Spec.Containers[0].Image = "busybox:failed"
			_, err = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Update(context.TODO(), cloneset, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			//wait 20 seconds
			ginkgo.By(fmt.Sprintf("waiting 20 seconds, and check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			time.Sleep(time.Second * 20)
			expectStatus = &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 0,
				DesiredAvailable:   1,
				CurrentAvailable:   1,
				TotalReplicas:      2,
			}
			setPubStatus(expectStatus)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus = &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			// check now pod
			pods, err := sidecarTester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			noUpdatePods := make([]corev1.Pod, 0)
			for _, pod := range pods {
				if pod.Spec.Containers[0].Image == "busybox:failed" || !pod.DeletionTimestamp.IsZero() {
					continue
				}
				noUpdatePods = append(noUpdatePods, *pod)
			}
			gomega.Expect(noUpdatePods).To(gomega.HaveLen(1))

			// update success image
			ginkgo.By(fmt.Sprintf("update CloneSet(%s.%s) success image(busybox:1.33)", cloneset.Namespace, cloneset.Name))
			cloneset, _ = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Get(context.TODO(), cloneset.Name, metav1.GetOptions{})
			cloneset.Spec.Template.Spec.Containers[0].Image = "busybox:1.33"
			cloneset, err = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Update(context.TODO(), cloneset, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			tester.WaitForCloneSetMinReadyAndRunning([]*appsv1alpha1.CloneSet{cloneset}, 1)

			//wait 20 seconds
			ginkgo.By(fmt.Sprintf("check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			expectStatus = &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 1,
				DesiredAvailable:   1,
				CurrentAvailable:   2,
				TotalReplicas:      2,
			}
			setPubStatus(expectStatus)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus = &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			//check pods
			pods, err = sidecarTester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			newPods := make([]corev1.Pod, 0)
			for _, pod := range pods {
				if !pod.DeletionTimestamp.IsZero() || pod.Spec.Containers[0].Image != "busybox:1.33" {
					continue
				}
				newPods = append(newPods, *pod)
			}
			gomega.Expect(newPods).To(gomega.HaveLen(2))
			ginkgo.By("PodUnavailableBudget selector cloneSet, update failed image and block done")
		})

		ginkgo.It("PodUnavailableBudget selector two cloneSets, strategy.type=in-place, update success image", func() {
			// create pub
			pub := tester.NewBasePub(ns)
			pub.Spec.MaxUnavailable = &intstr.IntOrString{
				Type:   intstr.String,
				StrVal: "20%",
			}
			ginkgo.By(fmt.Sprintf("Creating PodUnavailableBudget(%s.%s)", pub.Namespace, pub.Name))
			tester.CreatePub(pub)

			// create cloneset1
			cloneset := tester.NewBaseCloneSet(ns)
			cloneset.Spec.Replicas = utilpointer.Int32Ptr(5)
			cloneset.Spec.UpdateStrategy.Type = appsv1alpha1.InPlaceIfPossibleCloneSetUpdateStrategyType
			clonesetIn1 := cloneset.DeepCopy()
			clonesetIn1.Name = fmt.Sprintf("%s-1", clonesetIn1.Name)
			ginkgo.By(fmt.Sprintf("Creating CloneSet1(%s.%s)", clonesetIn1.Namespace, clonesetIn1.Name))
			clonesetIn1 = tester.CreateCloneSet(clonesetIn1)
			//create cloneSet2
			clonesetIn2 := cloneset.DeepCopy()
			clonesetIn2.Name = fmt.Sprintf("%s-2", clonesetIn2.Name)
			ginkgo.By(fmt.Sprintf("Creating CloneSet2(%s.%s)", clonesetIn2.Namespace, clonesetIn2.Name))
			clonesetIn2 = tester.CreateCloneSet(clonesetIn2)

			// wait 10 seconds
			time.Sleep(time.Second * 10)
			ginkgo.By(fmt.Sprintf("check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			expectStatus := &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 2,
				DesiredAvailable:   8,
				CurrentAvailable:   10,
				TotalReplicas:      10,
			}
			setPubStatus(expectStatus)
			pub, err := kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus := &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			// update failed image
			ginkgo.By(fmt.Sprintf("update CloneSet(%s.%s) with failed image(busybox:failed)", cloneset.Namespace, cloneset.Name))
			clonesetIn1.Spec.Template.Spec.Containers[0].Image = "busybox:failed"
			_, err = kc.AppsV1alpha1().CloneSets(clonesetIn1.Namespace).Update(context.TODO(), clonesetIn1, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			clonesetIn2.Spec.Template.Spec.Containers[0].Image = "busybox:failed"
			_, err = kc.AppsV1alpha1().CloneSets(clonesetIn2.Namespace).Update(context.TODO(), clonesetIn2, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			//wait 20 seconds
			ginkgo.By(fmt.Sprintf("waiting 20 seconds, and check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			time.Sleep(time.Second * 20)
			expectStatus = &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 0,
				DesiredAvailable:   8,
				CurrentAvailable:   8,
				TotalReplicas:      10,
			}
			setPubStatus(expectStatus)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus = &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			// check now pod
			pods, err := sidecarTester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			noUpdatePods := make([]corev1.Pod, 0)
			for _, pod := range pods {
				if pod.Spec.Containers[0].Image == "busybox:failed" || !pod.DeletionTimestamp.IsZero() {
					continue
				}
				noUpdatePods = append(noUpdatePods, *pod)
			}
			gomega.Expect(noUpdatePods).To(gomega.HaveLen(8))

			// update success image
			ginkgo.By(fmt.Sprintf("update CloneSet(%s.%s) success image(busybox:1.33)", cloneset.Namespace, cloneset.Name))
			clonesetIn1, _ = kc.AppsV1alpha1().CloneSets(clonesetIn1.Namespace).Get(context.TODO(), clonesetIn1.Name, metav1.GetOptions{})
			clonesetIn1.Spec.Template.Spec.Containers[0].Image = "busybox:1.33"
			clonesetIn1, err = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Update(context.TODO(), clonesetIn1, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// update success image
			clonesetIn2, _ = kc.AppsV1alpha1().CloneSets(clonesetIn2.Namespace).Get(context.TODO(), clonesetIn2.Name, metav1.GetOptions{})
			clonesetIn2.Spec.Template.Spec.Containers[0].Image = "busybox:1.33"
			clonesetIn2, err = kc.AppsV1alpha1().CloneSets(clonesetIn2.Namespace).Update(context.TODO(), clonesetIn2, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			tester.WaitForCloneSetMinReadyAndRunning([]*appsv1alpha1.CloneSet{clonesetIn1, clonesetIn2}, 7)

			// check pub status
			ginkgo.By(fmt.Sprintf("check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			expectStatus = &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 2,
				DesiredAvailable:   8,
				CurrentAvailable:   10,
				TotalReplicas:      10,
			}
			setPubStatus(expectStatus)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus = &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			//check pods
			pods, err = sidecarTester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			newPods := make([]corev1.Pod, 0)
			for _, pod := range pods {
				if pod.DeletionTimestamp.IsZero() && pod.Spec.Containers[0].Image == "busybox:1.33" {
					newPods = append(newPods, *pod)
				}
			}
			gomega.Expect(newPods).To(gomega.HaveLen(10))
			ginkgo.By("PodUnavailableBudget selector two cloneSets, strategy.type=in-place, update success image done")
		})

		ginkgo.It("PodUnavailableBudget selector cloneSet and sidecarSet, strategy.type=in-place, update success image", func() {
			// create pub
			pub := tester.NewBasePub(ns)
			pub.Spec.MaxUnavailable = &intstr.IntOrString{
				Type:   intstr.String,
				StrVal: "20%",
			}
			ginkgo.By(fmt.Sprintf("Creating PodUnavailableBudget(%s.%s)", pub.Namespace, pub.Name))
			tester.CreatePub(pub)

			// create sidecarSet
			sidecarSet := sidecarTester.NewBaseSidecarSet(ns)
			sidecarSet.Spec.Namespace = ns
			sidecarSet.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "busybox",
				},
			}
			sidecarSet.Spec.Containers = []appsv1alpha1.SidecarContainer{
				{
					Container: corev1.Container{
						Name:    "nginx-sidecar",
						Image:   "nginx:1.18",
						Command: []string{"tail", "-f", "/dev/null"},
					},
				},
			}
			sidecarSet.Spec.UpdateStrategy = appsv1alpha1.SidecarSetUpdateStrategy{
				Type: appsv1alpha1.RollingUpdateSidecarSetStrategyType,
				MaxUnavailable: &intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "100%",
				},
			}
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSet.Name))
			sidecarSet = sidecarTester.CreateSidecarSet(sidecarSet)

			// create cloneset
			cloneset := tester.NewBaseCloneSet(ns)
			cloneset.Spec.UpdateStrategy.Type = appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType
			cloneset.Spec.Replicas = utilpointer.Int32Ptr(10)
			ginkgo.By(fmt.Sprintf("Creating CloneSet(%s.%s)", cloneset.Namespace, cloneset.Name))
			cloneset = tester.CreateCloneSet(cloneset)

			time.Sleep(time.Second)
			// check sidecarSet inject sidecar container
			ginkgo.By(fmt.Sprintf("check sidecarSet inject sidecar container and pub status"))
			pods, err := sidecarTester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			pod := pods[0]
			gomega.Expect(pod.Spec.Containers).To(gomega.HaveLen(len(cloneset.Spec.Template.Spec.Containers) + len(sidecarSet.Spec.Containers)))

			// wait 10 seconds
			time.Sleep(time.Second * 5)
			ginkgo.By(fmt.Sprintf("check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			expectStatus := &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 2,
				DesiredAvailable:   8,
				CurrentAvailable:   10,
				TotalReplicas:      10,
			}
			setPubStatus(expectStatus)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus := &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			// update success image
			ginkgo.By(fmt.Sprintf("update CloneSet(%s.%s) success image(busybox:1.33)", cloneset.Namespace, cloneset.Name))
			cloneset, _ = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Get(context.TODO(), cloneset.Name, metav1.GetOptions{})
			cloneset.Spec.Template.Spec.Containers[0].Image = "busybox:1.33"
			cloneset, err = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Update(context.TODO(), cloneset, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// update sidecar container success image
			ginkgo.By(fmt.Sprintf("update sidecar container success image"))
			sidecarSet.Spec.Containers[0].Image = "nginx:1.19"
			sidecarTester.UpdateSidecarSet(sidecarSet)
			time.Sleep(time.Second)
			tester.WaitForCloneSetMinReadyAndRunning([]*appsv1alpha1.CloneSet{cloneset}, 2)

			//wait 20 seconds
			ginkgo.By(fmt.Sprintf("check PodUnavailableBudget(%s.%s) Status", pub.Namespace, pub.Name))
			expectStatus = &policyv1alpha1.PodUnavailableBudgetStatus{
				UnavailableAllowed: 2,
				DesiredAvailable:   8,
				CurrentAvailable:   10,
				TotalReplicas:      10,
			}
			setPubStatus(expectStatus)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nowStatus = &pub.Status
			setPubStatus(nowStatus)
			gomega.Expect(nowStatus).To(gomega.Equal(expectStatus))

			//check pods
			pods, err = sidecarTester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			newPods := make([]corev1.Pod, 0)
			for _, pod := range pods {
				if pod.DeletionTimestamp.IsZero() && pod.Spec.Containers[1].Image == "busybox:1.33" && pod.Spec.Containers[0].Image == "nginx:1.19" {
					newPods = append(newPods, *pod)
				}
			}
			gomega.Expect(newPods).To(gomega.HaveLen(10))
			ginkgo.By("PodUnavailableBudget selector cloneSet, update failed image and block done")
		})
	})
})

func setPubStatus(status *policyv1alpha1.PodUnavailableBudgetStatus) {
	status.DisruptedPods = nil
	status.UnavailablePods = nil
	status.ObservedGeneration = 0
}