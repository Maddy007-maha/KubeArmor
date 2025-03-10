// Copyright 2021 Authors of KubeArmor
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"

	kl "github.com/kubearmor/KubeArmor/KubeArmor/common"
	tp "github.com/kubearmor/KubeArmor/KubeArmor/types"
)

// ================ //
// == Pod Update == //
// ================ //

// UpdateEndPointWithPod Function
func (dm *KubeArmorDaemon) UpdateEndPointWithPod(action string, pod tp.K8sPod) {
	dm.EndPointsLock.Lock()
	defer dm.EndPointsLock.Unlock()

	if action == "ADDED" {
		// create a new endpoint

		newPoint := tp.EndPoint{}

		newPoint.NamespaceName = pod.Metadata["namespaceName"]
		newPoint.EndPointName = pod.Metadata["podName"]

		newPoint.Labels = []string{}
		newPoint.Identities = []string{}
		newPoint.Containers = []string{}
		newPoint.AppArmorProfiles = map[string]string{}

		newPoint.Identities = append(newPoint.Identities, "namespaceName="+pod.Metadata["namespaceName"])

		// update labels and identities
		for k, v := range pod.Labels {
			if !kl.ContainsElement(newPoint.Labels, k+"="+v) {
				newPoint.Labels = append(newPoint.Labels, k+"="+v)
			}

			if !kl.ContainsElement(newPoint.Identities, k+"="+v) {
				newPoint.Identities = append(newPoint.Identities, k+"="+v)
			}
		}

		// update container list
		for k := range pod.Containers {
			if !kl.ContainsElement(newPoint.Containers, k) {
				newPoint.Containers = append(newPoint.Containers, k)
			}
		}

		// update flags
		if pod.Annotations["kubearmor-policy"] == "enabled" {
			newPoint.PolicyEnabled = tp.KubeArmorPolicyEnabled
		} else if pod.Annotations["kubearmor-policy"] == "audited" {
			newPoint.PolicyEnabled = tp.KubeArmorPolicyAudited
		} else {
			newPoint.PolicyEnabled = tp.KubeArmorPolicyDisabled
		}

		// parse annotations and set flags
		for _, visibility := range strings.Split(pod.Annotations["kubearmor-visibility"], ",") {
			if visibility == "process" {
				newPoint.ProcessVisibilityEnabled = true
			} else if visibility == "file" {
				newPoint.FileVisibilityEnabled = true
			} else if visibility == "network" {
				newPoint.NetworkVisibilityEnabled = true
			} else if visibility == "capabilities" {
				newPoint.CapabilitiesVisibilityEnabled = true
			}
		}

		// update containers
		dm.ContainersLock.Lock()
		for _, containerID := range newPoint.Containers {
			container := dm.Containers[containerID]

			container.NamespaceName = newPoint.NamespaceName
			container.EndPointName = newPoint.EndPointName
			container.ContainerName = pod.Containers[containerID]

			container.PolicyEnabled = newPoint.PolicyEnabled

			container.ProcessVisibilityEnabled = newPoint.ProcessVisibilityEnabled
			container.FileVisibilityEnabled = newPoint.FileVisibilityEnabled
			container.NetworkVisibilityEnabled = newPoint.NetworkVisibilityEnabled
			container.CapabilitiesVisibilityEnabled = newPoint.CapabilitiesVisibilityEnabled

			newPoint.AppArmorProfiles[containerID] = container.AppArmorProfile

			dm.Containers[containerID] = container
		}
		dm.ContainersLock.Unlock()

		// update selinux profile names to the endpoint
		newPoint.SELinuxProfiles = map[string]string{}
		for k, v := range pod.Metadata {
			if strings.HasPrefix(k, "selinux-") {
				contName := strings.Split(k, "selinux-")[1]
				newPoint.SELinuxProfiles[contName] = v
			}
		}

		// update host-side volume mounted
		newPoint.HostVolumes = []tp.HostVolumeMount{}
		newPoint.HostVolumes = append(newPoint.HostVolumes, pod.HostVolumes...)

		// update security policies with the identities
		newPoint.SecurityPolicies = dm.GetSecurityPolicies(newPoint.Identities)

		// add the endpoint into the endpoint list
		dm.EndPoints = append(dm.EndPoints, newPoint)

		if newPoint.PolicyEnabled == tp.KubeArmorPolicyEnabled {
			// create and register security profiles
			dm.RuntimeEnforcer.UpdateSecurityProfiles(action, pod, true)
		}

		// update security policies
		dm.LogFeeder.UpdateSecurityPolicies(action, newPoint)

		// enforce security policies
		dm.RuntimeEnforcer.UpdateSecurityPolicies(newPoint)

	} else if action == "MODIFIED" {
		for idx, endPoint := range dm.EndPoints {
			if pod.Metadata["namespaceName"] == endPoint.NamespaceName && pod.Metadata["podName"] == endPoint.EndPointName {
				// update the labels and identities of the endpoint

				dm.EndPoints[idx].Labels = []string{}
				dm.EndPoints[idx].Identities = []string{}
				dm.EndPoints[idx].Containers = []string{}
				dm.EndPoints[idx].AppArmorProfiles = map[string]string{}

				dm.EndPoints[idx].Identities = append(dm.EndPoints[idx].Identities, "namespaceName="+pod.Metadata["namespaceName"])

				for k, v := range pod.Labels {
					if !kl.ContainsElement(dm.EndPoints[idx].Labels, k+"="+v) {
						dm.EndPoints[idx].Labels = append(dm.EndPoints[idx].Labels, k+"="+v)
					}

					if !kl.ContainsElement(dm.EndPoints[idx].Identities, k+"="+v) {
						dm.EndPoints[idx].Identities = append(dm.EndPoints[idx].Identities, k+"="+v)
					}
				}

				// update container list
				for k := range pod.Containers {
					if !kl.ContainsElement(dm.EndPoints[idx].Containers, k) {
						dm.EndPoints[idx].Containers = append(dm.EndPoints[idx].Containers, k)
					}
				}

				// update flags

				prevPolicyEnabled := dm.EndPoints[idx].PolicyEnabled

				if pod.Annotations["kubearmor-policy"] == "enabled" {
					dm.EndPoints[idx].PolicyEnabled = tp.KubeArmorPolicyEnabled
				} else if pod.Annotations["kubearmor-policy"] == "audited" {
					dm.EndPoints[idx].PolicyEnabled = tp.KubeArmorPolicyAudited
				} else {
					dm.EndPoints[idx].PolicyEnabled = tp.KubeArmorPolicyDisabled
				}

				// parse annotations and set flags

				dm.EndPoints[idx].ProcessVisibilityEnabled = false
				dm.EndPoints[idx].FileVisibilityEnabled = false
				dm.EndPoints[idx].NetworkVisibilityEnabled = false
				dm.EndPoints[idx].CapabilitiesVisibilityEnabled = false

				for _, visibility := range strings.Split(pod.Annotations["kubearmor-visibility"], ",") {
					if visibility == "process" {
						dm.EndPoints[idx].ProcessVisibilityEnabled = true
					} else if visibility == "file" {
						dm.EndPoints[idx].FileVisibilityEnabled = true
					} else if visibility == "network" {
						dm.EndPoints[idx].NetworkVisibilityEnabled = true
					} else if visibility == "capabilities" {
						dm.EndPoints[idx].CapabilitiesVisibilityEnabled = true
					}
				}

				// update containers
				dm.ContainersLock.Lock()
				for _, containerID := range dm.EndPoints[idx].Containers {
					container := dm.Containers[containerID]

					container.NamespaceName = dm.EndPoints[idx].NamespaceName
					container.EndPointName = dm.EndPoints[idx].EndPointName
					container.ContainerName = pod.Containers[containerID]

					container.PolicyEnabled = dm.EndPoints[idx].PolicyEnabled

					container.ProcessVisibilityEnabled = dm.EndPoints[idx].ProcessVisibilityEnabled
					container.FileVisibilityEnabled = dm.EndPoints[idx].FileVisibilityEnabled
					container.NetworkVisibilityEnabled = dm.EndPoints[idx].NetworkVisibilityEnabled
					container.CapabilitiesVisibilityEnabled = dm.EndPoints[idx].CapabilitiesVisibilityEnabled

					dm.EndPoints[idx].AppArmorProfiles[containerID] = container.AppArmorProfile

					dm.Containers[containerID] = container
				}
				dm.ContainersLock.Unlock()

				if prevPolicyEnabled != tp.KubeArmorPolicyEnabled && dm.EndPoints[idx].PolicyEnabled == tp.KubeArmorPolicyEnabled {
					// initialize and register security profiles
					dm.RuntimeEnforcer.UpdateSecurityProfiles("ADDED", pod, true)
				}

				// get security policies according to the updated identities
				dm.EndPoints[idx].SecurityPolicies = dm.GetSecurityPolicies(dm.EndPoints[idx].Identities)

				// update security policies
				dm.LogFeeder.UpdateSecurityPolicies(action, dm.EndPoints[idx])

				// enforce security policies
				dm.RuntimeEnforcer.UpdateSecurityPolicies(dm.EndPoints[idx])

				break
			}
		}

	} else { // DELETED
		for idx, endPoint := range dm.EndPoints {
			if pod.Metadata["namespaceName"] == endPoint.NamespaceName && pod.Metadata["podName"] == endPoint.EndPointName {
				if dm.EndPoints[idx].PolicyEnabled == tp.KubeArmorPolicyEnabled {
					// initialize and unregister security profiles
					dm.RuntimeEnforcer.UpdateSecurityProfiles(action, pod, true)
				}

				// remove endpoint
				dm.EndPoints = append(dm.EndPoints[:idx], dm.EndPoints[idx+1:]...)

				break
			}
		}
	}
}

// WatchK8sPods Function
func (dm *KubeArmorDaemon) WatchK8sPods() {
	for {
		if resp := K8s.WatchK8sPods(); resp != nil {
			defer resp.Body.Close()

			decoder := json.NewDecoder(resp.Body)
			for {
				event := tp.K8sPodEvent{}
				if err := decoder.Decode(&event); err == io.EOF {
					break
				} else if err != nil {
					break
				}

				// create a pod

				pod := tp.K8sPod{}

				pod.Metadata = map[string]string{}
				pod.Metadata["namespaceName"] = event.Object.ObjectMeta.Namespace
				pod.Metadata["podName"] = event.Object.ObjectMeta.Name

				if len(event.Object.ObjectMeta.OwnerReferences) > 0 {
					if event.Object.ObjectMeta.OwnerReferences[0].Kind == "ReplicaSet" {
						deploymentName := K8s.GetDeploymentNameControllingReplicaSet(pod.Metadata["namespaceName"], event.Object.ObjectMeta.OwnerReferences[0].Name)
						if deploymentName != "" {
							pod.Metadata["deploymentName"] = deploymentName
						}
					}
				}

				pod.Annotations = map[string]string{}
				for k, v := range event.Object.Annotations {
					pod.Annotations[k] = v
				}

				pod.Labels = map[string]string{}
				for k, v := range event.Object.Labels {
					if k == "pod-template-hash" {
						continue
					}

					if k == "pod-template-generation" {
						continue
					}

					if k == "controller-revision-hash" {
						continue
					}
					pod.Labels[k] = v
				}

				pod.Containers = map[string]string{}
				for _, container := range event.Object.Status.ContainerStatuses {
					if len(container.ContainerID) > 0 {
						if strings.HasPrefix(container.ContainerID, "docker://") {
							containerID := strings.TrimPrefix(container.ContainerID, "docker://")
							pod.Containers[containerID] = container.Name
						} else if strings.HasPrefix(container.ContainerID, "containerd://") {
							containerID := strings.TrimPrefix(container.ContainerID, "containerd://")
							pod.Containers[containerID] = container.Name
						}
					}
				}

				if dm.EnableEnforcerPerPod {
					if _, ok := pod.Annotations["kubearmor-policy"]; ok {
						if pod.Annotations["kubearmor-policy"] != "enabled" && pod.Annotations["kubearmor-policy"] != "disabled" {
							pod.Annotations["kubearmor-policy"] = "audited"
						}
					} else {
						pod.Annotations["kubearmor-policy"] = "audited"
					}
				} else { // EnableEnforcerAll
					if _, ok := pod.Annotations["kubearmor-policy"]; ok {
						if pod.Annotations["kubearmor-policy"] != "enabled" && pod.Annotations["kubearmor-policy"] != "disabled" && pod.Annotations["kubearmor-policy"] != "audited" {
							pod.Annotations["kubearmor-policy"] = "enabled"
						}
					} else {
						pod.Annotations["kubearmor-policy"] = "enabled"
					}
				}

				// == //

				if pod.Metadata["namespaceName"] == "kube-system" {
					// exception: kubernetes app
					if _, ok := pod.Labels["k8s-app"]; ok {
						pod.Annotations["kubearmor-policy"] = "audited"
					}

					// exception: cilium-operator
					if val, ok := pod.Labels["io.cilium/app"]; ok && val == "operator" {
						pod.Annotations["kubearmor-policy"] = "audited"
					}
				}

				// == //

				if dm.RuntimeEnforcer.IsEnabled() {
					if lsm, err := ioutil.ReadFile("/sys/kernel/security/lsm"); err == nil {
						// exception: no AppArmor
						if !strings.Contains(string(lsm), "apparmor") {
							if pod.Annotations["kubearmor-policy"] == "enabled" {
								pod.Annotations["kubearmor-policy"] = "audited"
							}
						}
					}
				} else { // No LSM
					if pod.Annotations["kubearmor-policy"] == "enabled" {
						pod.Annotations["kubearmor-policy"] = "audited"
					}
				}

				if _, ok := pod.Annotations["kubearmor-visibility"]; !ok {
					pod.Annotations["kubearmor-visibility"] = "none"
				}

				if event.Type == "ADDED" || event.Type == "MODIFIED" {
					exist := false

					dm.K8sPodsLock.Lock()
					for _, k8spod := range dm.K8sPods {
						if k8spod.Metadata["namespaceName"] == pod.Metadata["namespaceName"] && k8spod.Metadata["podName"] == pod.Metadata["podName"] {
							if k8spod.Annotations["kubearmor-policy"] == "patched" {
								exist = true
								break
							}
						}
					}
					dm.K8sPodsLock.Unlock()

					if exist {
						continue
					}
				}

				// == AppArmor == //

				if pod.Annotations["kubearmor-policy"] == "enabled" {
					appArmorAnnotations := map[string]string{}
					updateAppArmor := false

					for k, v := range pod.Annotations {
						if strings.HasPrefix(k, "container.apparmor.security.beta.kubernetes.io") {
							if v == "unconfined" {
								containerName := strings.Split(k, "/")[1]
								appArmorAnnotations[containerName] = v
							} else {
								containerName := strings.Split(k, "/")[1]
								appArmorAnnotations[containerName] = strings.Split(v, "/")[1]
							}
						}
					}

					for _, container := range event.Object.Spec.Containers {
						if _, ok := appArmorAnnotations[container.Name]; !ok {
							appArmorAnnotations[container.Name] = "kubearmor-" + pod.Metadata["namespaceName"] + "-" + container.Name
							updateAppArmor = true
						}
					}

					if dm.RuntimeEnforcer.GetEnforcerType() == "apparmor" {
						if updateAppArmor && (event.Type == "ADDED" || event.Type == "MODIFIED") {
							if deploymentName, ok := pod.Metadata["deploymentName"]; ok {
								if err := K8s.PatchDeploymentWithAppArmorAnnotations(pod.Metadata["namespaceName"], deploymentName, appArmorAnnotations); err != nil {
									dm.LogFeeder.Errf("Failed to update AppArmor Profiles (%s/%s/%s, %s)", pod.Metadata["namespaceName"], deploymentName, pod.Metadata["podName"], err.Error())
								} else {
									dm.LogFeeder.Printf("Patched AppArmor Profiles (%s/%s/%s)", pod.Metadata["namespaceName"], deploymentName, pod.Metadata["podName"])
								}
								pod.Annotations["kubearmor-policy"] = "patched"
							}
						}
					}
				}

				// == SELinux == //

				if pod.Annotations["kubearmor-policy"] == "enabled" {
					pod.HostVolumes = []tp.HostVolumeMount{}
					seLinuxContexts := map[string]string{}
					updateSELinux := false

					for _, v := range event.Object.Spec.Volumes {
						if v.HostPath != nil {
							hostVolume := tp.HostVolumeMount{}

							hostVolume.UsedByContainerReadOnly = map[string]bool{}
							hostVolume.UsedByContainerPath = map[string]string{}

							hostVolume.VolumeName = v.Name
							hostVolume.PathName = v.HostPath.Path
							hostVolume.Type = string(*v.HostPath.Type)

							pod.HostVolumes = append(pod.HostVolumes, hostVolume)
						}
					}

					for _, container := range event.Object.Spec.Containers {
						// match container volumes to host mounted volume
						for _, containerVolume := range container.VolumeMounts {
							for i, hostVoulme := range pod.HostVolumes {
								if containerVolume.Name == hostVoulme.VolumeName {
									if _, ok := pod.HostVolumes[i].UsedByContainerReadOnly[container.Name]; !ok {
										pod.HostVolumes[i].UsedByContainerReadOnly[container.Name] = containerVolume.ReadOnly
										pod.HostVolumes[i].UsedByContainerPath[container.Name] = containerVolume.MountPath
									}
								}
							}
						}

						if container.SecurityContext != nil && container.SecurityContext.SELinuxOptions != nil {
							if strings.Contains(container.SecurityContext.SELinuxOptions.Type, ".process") {
								if _, ok := pod.Metadata["selinux-"+container.Name]; !ok {
									selinuxContext := strings.Split(container.SecurityContext.SELinuxOptions.Type, ".process")[0]
									pod.Metadata["selinux-"+container.Name] = selinuxContext
								}
							}
						}
					}

					for _, container := range event.Object.Spec.Containers {
						if container.SecurityContext == nil || container.SecurityContext.SELinuxOptions == nil || container.SecurityContext.SELinuxOptions.Type == "" {
							if _, ok1 := seLinuxContexts[container.Name]; !ok1 {
								if _, ok2 := pod.Metadata["deploymentName"]; !ok2 {
									continue
								}

								container.SecurityContext = &v1.SecurityContext{
									SELinuxOptions: &v1.SELinuxOptions{
										Type: "kubearmor-" + pod.Metadata["namespaceName"] + "-" + pod.Metadata["deploymentName"] + "-" + container.Name + ".process",
									},
								}

								// clear container volume, if not delete volumeMounts, rolling update error
								container.VolumeMounts = []v1.VolumeMount{}

								b, _ := json.Marshal(container)
								seLinuxContexts[container.Name] = string(b)

								// set update flag
								updateSELinux = true
							}
						}
					}

					// if no selinux annotations but kubearmor-policy is enabled, add selinux annotations
					if dm.RuntimeEnforcer.GetEnforcerType() == "selinux" {
						if updateSELinux && (event.Type == "ADDED" || event.Type == "MODIFIED") {
							if deploymentName, ok := pod.Metadata["deploymentName"]; ok {
								if err := K8s.PatchDeploymentWithSELinuxOptions(pod.Metadata["namespaceName"], deploymentName, seLinuxContexts); err != nil {
									dm.LogFeeder.Errf("Failed to update SELinux security options (%s/%s/%s, %s)", pod.Metadata["namespaceName"], deploymentName, pod.Metadata["podName"], err.Error())
								} else {
									dm.LogFeeder.Printf("Patched SELinux security options (%s/%s/%s)", pod.Metadata["namespaceName"], deploymentName, pod.Metadata["podName"])
								}
								pod.Annotations["kubearmor-policy"] = "patched"
							}
						}
					}
				}

				// == //

				// update the pod into the pod list

				dm.K8sPodsLock.Lock()

				if event.Type == "ADDED" {
					if !kl.ContainsElement(dm.K8sPods, pod) {
						dm.K8sPods = append(dm.K8sPods, pod)
					}
				} else if event.Type == "MODIFIED" {
					for idx, k8spod := range dm.K8sPods {
						if k8spod.Metadata["namespaceName"] == pod.Metadata["namespaceName"] && k8spod.Metadata["podName"] == pod.Metadata["podName"] {
							dm.K8sPods[idx] = pod
							break
						}
					}
				} else if event.Type == "DELETED" {
					for idx, k8spod := range dm.K8sPods {
						if k8spod.Metadata["namespaceName"] == pod.Metadata["namespaceName"] && k8spod.Metadata["podName"] == pod.Metadata["podName"] {
							dm.K8sPods = append(dm.K8sPods[:idx], dm.K8sPods[idx+1:]...)
							break
						}
					}
				} else { // Otherwise
					dm.K8sPodsLock.Unlock()
					continue
				}

				dm.K8sPodsLock.Unlock()

				if pod.Annotations["kubearmor-policy"] != "patched" {
					dm.LogFeeder.Printf("Detected a Pod (%s/%s/%s)", strings.ToLower(event.Type), pod.Metadata["namespaceName"], pod.Metadata["podName"])
				}

				// update a endpoint corresponding to the pod
				dm.UpdateEndPointWithPod(event.Type, pod)
			}
		} else {
			time.Sleep(time.Second * 1)
		}
	}
}

// ============================ //
// == Security Policy Update == //
// ============================ //

// GetSecurityPolicies Function
func (dm *KubeArmorDaemon) GetSecurityPolicies(identities []string) []tp.SecurityPolicy {
	dm.SecurityPoliciesLock.Lock()
	defer dm.SecurityPoliciesLock.Unlock()

	secPolicies := []tp.SecurityPolicy{}

	for _, policy := range dm.SecurityPolicies {
		if kl.MatchIdentities(policy.Spec.Selector.Identities, identities) {
			secPolicy := tp.SecurityPolicy{}
			if err := kl.Clone(policy, &secPolicy); err != nil {
				fmt.Println("Failed to clone a policy")
			}
			secPolicies = append(secPolicies, secPolicy)
		}
	}

	return secPolicies
}

// UpdateSecurityPolicy Function
func (dm *KubeArmorDaemon) UpdateSecurityPolicy(action string, secPolicy tp.SecurityPolicy) {
	dm.EndPointsLock.Lock()
	defer dm.EndPointsLock.Unlock()

	for idx, endPoint := range dm.EndPoints {
		// update a security policy
		if kl.MatchIdentities(secPolicy.Spec.Selector.Identities, endPoint.Identities) {
			if action == "ADDED" {
				// add a new security policy if it doesn't exist
				if !kl.ContainsElement(endPoint.SecurityPolicies, secPolicy) {
					dm.EndPoints[idx].SecurityPolicies = append(dm.EndPoints[idx].SecurityPolicies, secPolicy)
				}
			} else if action == "MODIFIED" {
				for idxP, policy := range endPoint.SecurityPolicies {
					if policy.Metadata["namespaceName"] == secPolicy.Metadata["namespaceName"] && policy.Metadata["policyName"] == secPolicy.Metadata["policyName"] {
						dm.EndPoints[idx].SecurityPolicies[idxP] = secPolicy
						break
					}
				}
			} else if action == "DELETED" {
				// remove the given policy from the security policy list of this endpoint
				for idxP, policy := range endPoint.SecurityPolicies {
					if reflect.DeepEqual(secPolicy, policy) {
						dm.EndPoints[idx].SecurityPolicies = append(dm.EndPoints[idx].SecurityPolicies[:idxP], dm.EndPoints[idx].SecurityPolicies[idxP+1:]...)
						break
					}
				}
			}

			// update security policies
			dm.LogFeeder.UpdateSecurityPolicies("UPDATED", dm.EndPoints[idx])

			// enforce security policies
			dm.RuntimeEnforcer.UpdateSecurityPolicies(dm.EndPoints[idx])
		}
	}
}

// WatchSecurityPolicies Function
func (dm *KubeArmorDaemon) WatchSecurityPolicies() {
	for {
		if !K8s.CheckCustomResourceDefinition("kubearmorpolicies") {
			time.Sleep(time.Second * 1)
			continue
		}

		if resp := K8s.WatchK8sSecurityPolicies(); resp != nil {
			defer resp.Body.Close()

			decoder := json.NewDecoder(resp.Body)
			for {
				event := tp.K8sKubeArmorPolicyEvent{}
				if err := decoder.Decode(&event); err == io.EOF {
					break
				} else if err != nil {
					break
				}

				if event.Object.Status.Status != "" && event.Object.Status.Status != "OK" {
					continue
				}

				dm.SecurityPoliciesLock.Lock()

				// create a security policy

				secPolicy := tp.SecurityPolicy{}

				secPolicy.Metadata = map[string]string{}
				secPolicy.Metadata["namespaceName"] = event.Object.Metadata.Namespace
				secPolicy.Metadata["policyName"] = event.Object.Metadata.Name

				if err := kl.Clone(event.Object.Spec, &secPolicy.Spec); err != nil {
					fmt.Println("Failed to clone a spec")
				}

				kl.ObjCommaExpandFirstDupOthers(&secPolicy.Spec.Network.MatchProtocols)
				kl.ObjCommaExpandFirstDupOthers(&secPolicy.Spec.Capabilities.MatchCapabilities)

				if secPolicy.Spec.Severity == 0 {
					secPolicy.Spec.Severity = 1 // the lowest severity, by default
				}

				switch secPolicy.Spec.Action {
				case "allow":
					secPolicy.Spec.Action = "Allow"
				case "audit":
					secPolicy.Spec.Action = "Audit"
				case "block":
					secPolicy.Spec.Action = "Block"
				case "":
					secPolicy.Spec.Action = "Block" // by default
				}

				// add identities

				secPolicy.Spec.Selector.Identities = append(secPolicy.Spec.Selector.Identities, "namespaceName="+event.Object.Metadata.Namespace)

				for k, v := range secPolicy.Spec.Selector.MatchLabels {
					if !kl.ContainsElement(secPolicy.Spec.Selector.Identities, k+"="+v) {
						secPolicy.Spec.Selector.Identities = append(secPolicy.Spec.Selector.Identities, k+"="+v)
					}
				}

				// add severities, tags, messages, and actions

				if len(secPolicy.Spec.Process.MatchPaths) > 0 {
					for idx, path := range secPolicy.Spec.Process.MatchPaths {
						if path.Severity == 0 {
							if secPolicy.Spec.Process.Severity != 0 {
								secPolicy.Spec.Process.MatchPaths[idx].Severity = secPolicy.Spec.Process.Severity
							} else {
								secPolicy.Spec.Process.MatchPaths[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(path.Tags) == 0 {
							if len(secPolicy.Spec.Process.Tags) > 0 {
								secPolicy.Spec.Process.MatchPaths[idx].Tags = secPolicy.Spec.Process.Tags
							} else {
								secPolicy.Spec.Process.MatchPaths[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(path.Message) == 0 {
							if len(secPolicy.Spec.Process.Message) > 0 {
								secPolicy.Spec.Process.MatchPaths[idx].Message = secPolicy.Spec.Process.Message
							} else {
								secPolicy.Spec.Process.MatchPaths[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(path.Action) == 0 {
							if len(secPolicy.Spec.Process.Action) > 0 {
								secPolicy.Spec.Process.MatchPaths[idx].Action = secPolicy.Spec.Process.Action
							} else {
								secPolicy.Spec.Process.MatchPaths[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				}

				if len(secPolicy.Spec.Process.MatchDirectories) > 0 {
					for idx, dir := range secPolicy.Spec.Process.MatchDirectories {
						if dir.Severity == 0 {
							if secPolicy.Spec.Process.Severity != 0 {
								secPolicy.Spec.Process.MatchDirectories[idx].Severity = secPolicy.Spec.Process.Severity
							} else {
								secPolicy.Spec.Process.MatchDirectories[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(dir.Tags) == 0 {
							if len(secPolicy.Spec.Process.Tags) > 0 {
								secPolicy.Spec.Process.MatchDirectories[idx].Tags = secPolicy.Spec.Process.Tags
							} else {
								secPolicy.Spec.Process.MatchDirectories[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(dir.Message) == 0 {
							if len(secPolicy.Spec.Process.Message) > 0 {
								secPolicy.Spec.Process.MatchDirectories[idx].Message = secPolicy.Spec.Process.Message
							} else {
								secPolicy.Spec.Process.MatchDirectories[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(dir.Action) == 0 {
							if len(secPolicy.Spec.Process.Action) > 0 {
								secPolicy.Spec.Process.MatchDirectories[idx].Action = secPolicy.Spec.Process.Action
							} else {
								secPolicy.Spec.Process.MatchDirectories[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				}

				if len(secPolicy.Spec.Process.MatchPatterns) > 0 {
					for idx, pat := range secPolicy.Spec.Process.MatchPatterns {
						if pat.Severity == 0 {
							if secPolicy.Spec.Process.Severity != 0 {
								secPolicy.Spec.Process.MatchPatterns[idx].Severity = secPolicy.Spec.Process.Severity
							} else {
								secPolicy.Spec.Process.MatchPatterns[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(pat.Tags) == 0 {
							if len(secPolicy.Spec.Process.Tags) > 0 {
								secPolicy.Spec.Process.MatchPatterns[idx].Tags = secPolicy.Spec.Process.Tags
							} else {
								secPolicy.Spec.Process.MatchPatterns[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(pat.Message) == 0 {
							if len(secPolicy.Spec.Process.Message) > 0 {
								secPolicy.Spec.Process.MatchPatterns[idx].Message = secPolicy.Spec.Process.Message
							} else {
								secPolicy.Spec.Process.MatchPatterns[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(pat.Action) == 0 {
							if len(secPolicy.Spec.Process.Action) > 0 {
								secPolicy.Spec.Process.MatchPatterns[idx].Action = secPolicy.Spec.Process.Action
							} else {
								secPolicy.Spec.Process.MatchPatterns[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				}

				if len(secPolicy.Spec.File.MatchPaths) > 0 {
					for idx, path := range secPolicy.Spec.File.MatchPaths {
						if path.Severity == 0 {
							if secPolicy.Spec.File.Severity != 0 {
								secPolicy.Spec.File.MatchPaths[idx].Severity = secPolicy.Spec.File.Severity
							} else {
								secPolicy.Spec.File.MatchPaths[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(path.Tags) == 0 {
							if len(secPolicy.Spec.File.Tags) > 0 {
								secPolicy.Spec.File.MatchPaths[idx].Tags = secPolicy.Spec.File.Tags
							} else {
								secPolicy.Spec.File.MatchPaths[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(path.Message) == 0 {
							if len(secPolicy.Spec.File.Message) > 0 {
								secPolicy.Spec.File.MatchPaths[idx].Message = secPolicy.Spec.File.Message
							} else {
								secPolicy.Spec.File.MatchPaths[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(path.Action) == 0 {
							if len(secPolicy.Spec.File.Action) > 0 {
								secPolicy.Spec.File.MatchPaths[idx].Action = secPolicy.Spec.File.Action
							} else {
								secPolicy.Spec.File.MatchPaths[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				}

				if len(secPolicy.Spec.File.MatchDirectories) > 0 {
					for idx, dir := range secPolicy.Spec.File.MatchDirectories {
						if dir.Severity == 0 {
							if secPolicy.Spec.File.Severity != 0 {
								secPolicy.Spec.File.MatchDirectories[idx].Severity = secPolicy.Spec.File.Severity
							} else {
								secPolicy.Spec.File.MatchDirectories[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(dir.Tags) == 0 {
							if len(secPolicy.Spec.File.Tags) > 0 {
								secPolicy.Spec.File.MatchDirectories[idx].Tags = secPolicy.Spec.File.Tags
							} else {
								secPolicy.Spec.File.MatchDirectories[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(dir.Message) == 0 {
							if len(secPolicy.Spec.File.Message) > 0 {
								secPolicy.Spec.File.MatchDirectories[idx].Message = secPolicy.Spec.File.Message
							} else {
								secPolicy.Spec.File.MatchDirectories[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(dir.Action) == 0 {
							if len(secPolicy.Spec.File.Action) > 0 {
								secPolicy.Spec.File.MatchDirectories[idx].Action = secPolicy.Spec.File.Action
							} else {
								secPolicy.Spec.File.MatchDirectories[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				}

				if len(secPolicy.Spec.File.MatchPatterns) > 0 {
					for idx, pat := range secPolicy.Spec.File.MatchPatterns {
						if pat.Severity == 0 {
							if secPolicy.Spec.File.Severity != 0 {
								secPolicy.Spec.File.MatchPatterns[idx].Severity = secPolicy.Spec.File.Severity
							} else {
								secPolicy.Spec.File.MatchPatterns[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(pat.Tags) == 0 {
							if len(secPolicy.Spec.File.Tags) > 0 {
								secPolicy.Spec.File.MatchPatterns[idx].Tags = secPolicy.Spec.File.Tags
							} else {
								secPolicy.Spec.File.MatchPatterns[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(pat.Message) == 0 {
							if len(secPolicy.Spec.File.Message) > 0 {
								secPolicy.Spec.File.MatchPatterns[idx].Message = secPolicy.Spec.File.Message
							} else {
								secPolicy.Spec.File.MatchPatterns[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(pat.Action) == 0 {
							if len(secPolicy.Spec.File.Action) > 0 {
								secPolicy.Spec.File.MatchPatterns[idx].Action = secPolicy.Spec.File.Action
							} else {
								secPolicy.Spec.File.MatchPatterns[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				}

				if len(secPolicy.Spec.Network.MatchProtocols) > 0 {
					for idx, proto := range secPolicy.Spec.Network.MatchProtocols {
						if proto.Severity == 0 {
							if secPolicy.Spec.Network.Severity != 0 {
								secPolicy.Spec.Network.MatchProtocols[idx].Severity = secPolicy.Spec.Network.Severity
							} else {
								secPolicy.Spec.Network.MatchProtocols[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(proto.Tags) == 0 {
							if len(secPolicy.Spec.Network.Tags) > 0 {
								secPolicy.Spec.Network.MatchProtocols[idx].Tags = secPolicy.Spec.Network.Tags
							} else {
								secPolicy.Spec.Network.MatchProtocols[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(proto.Message) == 0 {
							if len(secPolicy.Spec.Network.Message) > 0 {
								secPolicy.Spec.Network.MatchProtocols[idx].Message = secPolicy.Spec.Network.Message
							} else {
								secPolicy.Spec.Network.MatchProtocols[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(proto.Action) == 0 {
							if len(secPolicy.Spec.Network.Action) > 0 {
								secPolicy.Spec.Network.MatchProtocols[idx].Action = secPolicy.Spec.Network.Action
							} else {
								secPolicy.Spec.Network.MatchProtocols[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				}

				if len(secPolicy.Spec.Capabilities.MatchCapabilities) > 0 {
					for idx, cap := range secPolicy.Spec.Capabilities.MatchCapabilities {
						if cap.Severity == 0 {
							if secPolicy.Spec.Capabilities.Severity != 0 {
								secPolicy.Spec.Capabilities.MatchCapabilities[idx].Severity = secPolicy.Spec.Capabilities.Severity
							} else {
								secPolicy.Spec.Capabilities.MatchCapabilities[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(cap.Tags) == 0 {
							if len(secPolicy.Spec.Capabilities.Tags) > 0 {
								secPolicy.Spec.Capabilities.MatchCapabilities[idx].Tags = secPolicy.Spec.Capabilities.Tags
							} else {
								secPolicy.Spec.Capabilities.MatchCapabilities[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(cap.Message) == 0 {
							if len(secPolicy.Spec.Capabilities.Message) > 0 {
								secPolicy.Spec.Capabilities.MatchCapabilities[idx].Message = secPolicy.Spec.Capabilities.Message
							} else {
								secPolicy.Spec.Capabilities.MatchCapabilities[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(cap.Action) == 0 {
							if len(secPolicy.Spec.Capabilities.Action) > 0 {
								secPolicy.Spec.Capabilities.MatchCapabilities[idx].Action = secPolicy.Spec.Capabilities.Action
							} else {
								secPolicy.Spec.Capabilities.MatchCapabilities[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				}

				if len(secPolicy.Spec.SELinux.MatchVolumeMounts) > 0 {
					for idx, se := range secPolicy.Spec.SELinux.MatchVolumeMounts {
						if se.Severity == 0 {
							if secPolicy.Spec.SELinux.Severity != 0 {
								secPolicy.Spec.SELinux.MatchVolumeMounts[idx].Severity = secPolicy.Spec.SELinux.Severity
							} else {
								secPolicy.Spec.SELinux.MatchVolumeMounts[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(se.Tags) == 0 {
							if len(secPolicy.Spec.SELinux.Tags) > 0 {
								secPolicy.Spec.SELinux.MatchVolumeMounts[idx].Tags = secPolicy.Spec.SELinux.Tags
							} else {
								secPolicy.Spec.SELinux.MatchVolumeMounts[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(se.Message) == 0 {
							if len(secPolicy.Spec.SELinux.Message) > 0 {
								secPolicy.Spec.SELinux.MatchVolumeMounts[idx].Message = secPolicy.Spec.SELinux.Message
							} else {
								secPolicy.Spec.SELinux.MatchVolumeMounts[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(se.Action) == 0 {
							if len(secPolicy.Spec.SELinux.Action) > 0 {
								secPolicy.Spec.SELinux.MatchVolumeMounts[idx].Action = secPolicy.Spec.SELinux.Action
							} else {
								secPolicy.Spec.SELinux.MatchVolumeMounts[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				}

				// update a security policy into the policy list

				if event.Type == "ADDED" {
					if !kl.ContainsElement(dm.SecurityPolicies, secPolicy) {
						dm.SecurityPolicies = append(dm.SecurityPolicies, secPolicy)
					}
				} else if event.Type == "MODIFIED" {
					for idx, policy := range dm.SecurityPolicies {
						if policy.Metadata["namespaceName"] == secPolicy.Metadata["namespaceName"] && policy.Metadata["policyName"] == secPolicy.Metadata["policyName"] {
							dm.SecurityPolicies[idx] = secPolicy
							break
						}
					}
				} else if event.Type == "DELETED" {
					for idx, policy := range dm.SecurityPolicies {
						if reflect.DeepEqual(secPolicy, policy) {
							dm.SecurityPolicies = append(dm.SecurityPolicies[:idx], dm.SecurityPolicies[idx+1:]...)
							break
						}
					}
				}

				dm.SecurityPoliciesLock.Unlock()

				dm.LogFeeder.Printf("Detected a Security Policy (%s/%s/%s)", strings.ToLower(event.Type), secPolicy.Metadata["namespaceName"], secPolicy.Metadata["policyName"])

				// apply security policies to containers
				dm.UpdateSecurityPolicy(event.Type, secPolicy)
			}
		}
	}
}

// UpdateHostSecurityPolicies Function
func (dm *KubeArmorDaemon) UpdateHostSecurityPolicies() {
	// get node identities
	nodeIdentities := K8s.GetNodeIdentities()

	dm.HostSecurityPoliciesLock.Lock()
	defer dm.HostSecurityPoliciesLock.Unlock()

	secPolicies := []tp.HostSecurityPolicy{}

	for _, policy := range dm.HostSecurityPolicies {
		if kl.MatchIdentities(policy.Spec.NodeSelector.Identities, nodeIdentities) {
			secPolicies = append(secPolicies, policy)
		}
	}

	// update host security policies
	dm.LogFeeder.UpdateHostSecurityPolicies("UPDATED", secPolicies)

	// enforce host security policies
	dm.RuntimeEnforcer.UpdateHostSecurityPolicies(secPolicies)
}

// WatchHostSecurityPolicies Function
func (dm *KubeArmorDaemon) WatchHostSecurityPolicies() {
	for {
		if !K8s.CheckCustomResourceDefinition("kubearmorhostpolicies") {
			time.Sleep(time.Second * 1)
			continue
		}

		if resp := K8s.WatchK8sHostSecurityPolicies(); resp != nil {
			defer resp.Body.Close()

			decoder := json.NewDecoder(resp.Body)
			for {
				event := tp.K8sKubeArmorHostPolicyEvent{}
				if err := decoder.Decode(&event); err == io.EOF {
					break
				} else if err != nil {
					break
				}

				if event.Object.Status.Status != "" && event.Object.Status.Status != "OK" {
					continue
				}

				dm.HostSecurityPoliciesLock.Lock()

				// create a host security policy

				secPolicy := tp.HostSecurityPolicy{}

				secPolicy.Metadata = map[string]string{}
				secPolicy.Metadata["policyName"] = event.Object.Metadata.Name

				if err := kl.Clone(event.Object.Spec, &secPolicy.Spec); err != nil {
					fmt.Println("Failed to clone a spec")
				}

				kl.ObjCommaExpandFirstDupOthers(&secPolicy.Spec.Network.MatchProtocols)
				kl.ObjCommaExpandFirstDupOthers(&secPolicy.Spec.Capabilities.MatchCapabilities)

				if secPolicy.Spec.Severity == 0 {
					secPolicy.Spec.Severity = 1 // the lowest severity, by default
				}

				switch secPolicy.Spec.Action {
				case "allow":
					secPolicy.Spec.Action = "Allow"
				case "audit":
					secPolicy.Spec.Action = "Audit"
				case "block":
					secPolicy.Spec.Action = "Block"
				case "":
					secPolicy.Spec.Action = "Block" // by default
				}

				// add identities

				for k, v := range secPolicy.Spec.NodeSelector.MatchLabels {
					if !kl.ContainsElement(secPolicy.Spec.NodeSelector.Identities, k+"="+v) {
						secPolicy.Spec.NodeSelector.Identities = append(secPolicy.Spec.NodeSelector.Identities, k+"="+v)
					}
				}

				// add severities, tags, messages, and actions

				if len(secPolicy.Spec.Process.MatchPaths) > 0 {
					for idx, path := range secPolicy.Spec.Process.MatchPaths {
						if path.Severity == 0 {
							if secPolicy.Spec.Process.Severity != 0 {
								secPolicy.Spec.Process.MatchPaths[idx].Severity = secPolicy.Spec.Process.Severity
							} else {
								secPolicy.Spec.Process.MatchPaths[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(path.Tags) == 0 {
							if len(secPolicy.Spec.Process.Tags) > 0 {
								secPolicy.Spec.Process.MatchPaths[idx].Tags = secPolicy.Spec.Process.Tags
							} else {
								secPolicy.Spec.Process.MatchPaths[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(path.Message) == 0 {
							if len(secPolicy.Spec.Process.Message) > 0 {
								secPolicy.Spec.Process.MatchPaths[idx].Message = secPolicy.Spec.Process.Message
							} else {
								secPolicy.Spec.Process.MatchPaths[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(path.Action) == 0 {
							if len(secPolicy.Spec.Process.Action) > 0 {
								secPolicy.Spec.Process.MatchPaths[idx].Action = secPolicy.Spec.Process.Action
							} else {
								secPolicy.Spec.Process.MatchPaths[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				} else if len(secPolicy.Spec.Process.MatchDirectories) > 0 {
					for idx, dir := range secPolicy.Spec.Process.MatchDirectories {
						if dir.Severity == 0 {
							if secPolicy.Spec.Process.Severity != 0 {
								secPolicy.Spec.Process.MatchDirectories[idx].Severity = secPolicy.Spec.Process.Severity
							} else {
								secPolicy.Spec.Process.MatchDirectories[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(dir.Tags) == 0 {
							if len(secPolicy.Spec.Process.Tags) > 0 {
								secPolicy.Spec.Process.MatchDirectories[idx].Tags = secPolicy.Spec.Process.Tags
							} else {
								secPolicy.Spec.Process.MatchDirectories[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(dir.Message) == 0 {
							if len(secPolicy.Spec.Process.Message) > 0 {
								secPolicy.Spec.Process.MatchDirectories[idx].Message = secPolicy.Spec.Process.Message
							} else {
								secPolicy.Spec.Process.MatchDirectories[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(dir.Action) == 0 {
							if len(secPolicy.Spec.Process.Action) > 0 {
								secPolicy.Spec.Process.MatchDirectories[idx].Action = secPolicy.Spec.Process.Action
							} else {
								secPolicy.Spec.Process.MatchDirectories[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				} else if len(secPolicy.Spec.Process.MatchPatterns) > 0 {
					for idx, pat := range secPolicy.Spec.Process.MatchPatterns {
						if pat.Severity == 0 {
							if secPolicy.Spec.Process.Severity != 0 {
								secPolicy.Spec.Process.MatchPatterns[idx].Severity = secPolicy.Spec.Process.Severity
							} else {
								secPolicy.Spec.Process.MatchPatterns[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(pat.Tags) == 0 {
							if len(secPolicy.Spec.Process.Tags) > 0 {
								secPolicy.Spec.Process.MatchPatterns[idx].Tags = secPolicy.Spec.Process.Tags
							} else {
								secPolicy.Spec.Process.MatchPatterns[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(pat.Message) == 0 {
							if len(secPolicy.Spec.Process.Message) > 0 {
								secPolicy.Spec.Process.MatchPatterns[idx].Message = secPolicy.Spec.Process.Message
							} else {
								secPolicy.Spec.Process.MatchPatterns[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(pat.Action) == 0 {
							if len(secPolicy.Spec.Process.Action) > 0 {
								secPolicy.Spec.Process.MatchPatterns[idx].Action = secPolicy.Spec.Process.Action
							} else {
								secPolicy.Spec.Process.MatchPatterns[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				}

				if len(secPolicy.Spec.File.MatchPaths) > 0 {
					for idx, path := range secPolicy.Spec.File.MatchPaths {
						if path.Severity == 0 {
							if secPolicy.Spec.File.Severity != 0 {
								secPolicy.Spec.File.MatchPaths[idx].Severity = secPolicy.Spec.File.Severity
							} else {
								secPolicy.Spec.File.MatchPaths[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(path.Tags) == 0 {
							if len(secPolicy.Spec.File.Tags) > 0 {
								secPolicy.Spec.File.MatchPaths[idx].Tags = secPolicy.Spec.File.Tags
							} else {
								secPolicy.Spec.File.MatchPaths[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(path.Message) == 0 {
							if len(secPolicy.Spec.File.Message) > 0 {
								secPolicy.Spec.File.MatchPaths[idx].Message = secPolicy.Spec.File.Message
							} else {
								secPolicy.Spec.File.MatchPaths[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(path.Action) == 0 {
							if len(secPolicy.Spec.File.Action) > 0 {
								secPolicy.Spec.File.MatchPaths[idx].Action = secPolicy.Spec.File.Action
							} else {
								secPolicy.Spec.File.MatchPaths[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				} else if len(secPolicy.Spec.File.MatchDirectories) > 0 {
					for idx, dir := range secPolicy.Spec.File.MatchDirectories {
						if dir.Severity == 0 {
							if secPolicy.Spec.File.Severity != 0 {
								secPolicy.Spec.File.MatchDirectories[idx].Severity = secPolicy.Spec.File.Severity
							} else {
								secPolicy.Spec.File.MatchDirectories[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(dir.Tags) == 0 {
							if len(secPolicy.Spec.File.Tags) > 0 {
								secPolicy.Spec.File.MatchDirectories[idx].Tags = secPolicy.Spec.File.Tags
							} else {
								secPolicy.Spec.File.MatchDirectories[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(dir.Message) == 0 {
							if len(secPolicy.Spec.File.Message) > 0 {
								secPolicy.Spec.File.MatchDirectories[idx].Message = secPolicy.Spec.File.Message
							} else {
								secPolicy.Spec.File.MatchDirectories[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(dir.Action) == 0 {
							if len(secPolicy.Spec.File.Action) > 0 {
								secPolicy.Spec.File.MatchDirectories[idx].Action = secPolicy.Spec.File.Action
							} else {
								secPolicy.Spec.File.MatchDirectories[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				} else if len(secPolicy.Spec.File.MatchPatterns) > 0 {
					for idx, pat := range secPolicy.Spec.File.MatchPatterns {
						if pat.Severity == 0 {
							if secPolicy.Spec.File.Severity != 0 {
								secPolicy.Spec.File.MatchPatterns[idx].Severity = secPolicy.Spec.File.Severity
							} else {
								secPolicy.Spec.File.MatchPatterns[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(pat.Tags) == 0 {
							if len(secPolicy.Spec.File.Tags) > 0 {
								secPolicy.Spec.File.MatchPatterns[idx].Tags = secPolicy.Spec.File.Tags
							} else {
								secPolicy.Spec.File.MatchPatterns[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(pat.Message) == 0 {
							if len(secPolicy.Spec.File.Message) > 0 {
								secPolicy.Spec.File.MatchPatterns[idx].Message = secPolicy.Spec.File.Message
							} else {
								secPolicy.Spec.File.MatchPatterns[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(pat.Action) == 0 {
							if len(secPolicy.Spec.File.Action) > 0 {
								secPolicy.Spec.File.MatchPatterns[idx].Action = secPolicy.Spec.File.Action
							} else {
								secPolicy.Spec.File.MatchPatterns[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				}

				if len(secPolicy.Spec.Network.MatchProtocols) > 0 {
					for idx, proto := range secPolicy.Spec.Network.MatchProtocols {
						if proto.Severity == 0 {
							if secPolicy.Spec.Network.Severity != 0 {
								secPolicy.Spec.Network.MatchProtocols[idx].Severity = secPolicy.Spec.Network.Severity
							} else {
								secPolicy.Spec.Network.MatchProtocols[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(proto.Tags) == 0 {
							if len(secPolicy.Spec.Network.Tags) > 0 {
								secPolicy.Spec.Network.MatchProtocols[idx].Tags = secPolicy.Spec.Network.Tags
							} else {
								secPolicy.Spec.Network.MatchProtocols[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(proto.Message) == 0 {
							if len(secPolicy.Spec.Network.Message) > 0 {
								secPolicy.Spec.Network.MatchProtocols[idx].Message = secPolicy.Spec.Network.Message
							} else {
								secPolicy.Spec.Network.MatchProtocols[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(proto.Action) == 0 {
							if len(secPolicy.Spec.Network.Action) > 0 {
								secPolicy.Spec.Network.MatchProtocols[idx].Action = secPolicy.Spec.Network.Action
							} else {
								secPolicy.Spec.Network.MatchProtocols[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				}

				if len(secPolicy.Spec.Capabilities.MatchCapabilities) > 0 {
					for idx, cap := range secPolicy.Spec.Capabilities.MatchCapabilities {
						if cap.Severity == 0 {
							if secPolicy.Spec.Capabilities.Severity != 0 {
								secPolicy.Spec.Capabilities.MatchCapabilities[idx].Severity = secPolicy.Spec.Capabilities.Severity
							} else {
								secPolicy.Spec.Capabilities.MatchCapabilities[idx].Severity = secPolicy.Spec.Severity
							}
						}

						if len(cap.Tags) == 0 {
							if len(secPolicy.Spec.Capabilities.Tags) > 0 {
								secPolicy.Spec.Capabilities.MatchCapabilities[idx].Tags = secPolicy.Spec.Capabilities.Tags
							} else {
								secPolicy.Spec.Capabilities.MatchCapabilities[idx].Tags = secPolicy.Spec.Tags
							}
						}

						if len(cap.Message) == 0 {
							if len(secPolicy.Spec.Capabilities.Message) > 0 {
								secPolicy.Spec.Capabilities.MatchCapabilities[idx].Message = secPolicy.Spec.Capabilities.Message
							} else {
								secPolicy.Spec.Capabilities.MatchCapabilities[idx].Message = secPolicy.Spec.Message
							}
						}

						if len(cap.Action) == 0 {
							if len(secPolicy.Spec.Capabilities.Action) > 0 {
								secPolicy.Spec.Capabilities.MatchCapabilities[idx].Action = secPolicy.Spec.Capabilities.Action
							} else {
								secPolicy.Spec.Capabilities.MatchCapabilities[idx].Action = secPolicy.Spec.Action
							}
						}
					}
				}

				// update a security policy into the policy list

				if event.Type == "ADDED" {
					if !kl.ContainsElement(dm.HostSecurityPolicies, secPolicy) {
						dm.HostSecurityPolicies = append(dm.HostSecurityPolicies, secPolicy)
					}
				} else if event.Type == "MODIFIED" {
					for idx, policy := range dm.HostSecurityPolicies {
						if policy.Metadata["policyName"] == secPolicy.Metadata["policyName"] {
							dm.HostSecurityPolicies[idx] = secPolicy
							break
						}
					}
				} else if event.Type == "DELETED" {
					for idx, policy := range dm.HostSecurityPolicies {
						if reflect.DeepEqual(secPolicy, policy) {
							dm.HostSecurityPolicies = append(dm.HostSecurityPolicies[:idx], dm.HostSecurityPolicies[idx+1:]...)
							break
						}
					}
				}

				dm.HostSecurityPoliciesLock.Unlock()

				dm.LogFeeder.Printf("Detected a Host Security Policy (%s/%s)", strings.ToLower(event.Type), secPolicy.Metadata["policyName"])

				// apply security policies to a host
				dm.UpdateHostSecurityPolicies()
			}
		}
	}
}
