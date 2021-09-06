//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Copyright 2021 Authors of KubeArmor
// SPDX-License-Identifier: Apache-2.0

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeArmorExternalWorkloadPolicy) DeepCopyInto(out *KubeArmorExternalWorkloadPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeArmorExternalWorkloadPolicy.
func (in *KubeArmorExternalWorkloadPolicy) DeepCopy() *KubeArmorExternalWorkloadPolicy {
	if in == nil {
		return nil
	}
	out := new(KubeArmorExternalWorkloadPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *KubeArmorExternalWorkloadPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeArmorExternalWorkloadPolicyList) DeepCopyInto(out *KubeArmorExternalWorkloadPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]KubeArmorExternalWorkloadPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeArmorExternalWorkloadPolicyList.
func (in *KubeArmorExternalWorkloadPolicyList) DeepCopy() *KubeArmorExternalWorkloadPolicyList {
	if in == nil {
		return nil
	}
	out := new(KubeArmorExternalWorkloadPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *KubeArmorExternalWorkloadPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeArmorExternalWorkloadPolicySpec) DeepCopyInto(out *KubeArmorExternalWorkloadPolicySpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeArmorExternalWorkloadPolicySpec.
func (in *KubeArmorExternalWorkloadPolicySpec) DeepCopy() *KubeArmorExternalWorkloadPolicySpec {
	if in == nil {
		return nil
	}
	out := new(KubeArmorExternalWorkloadPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeArmorExternalWorkloadPolicyStatus) DeepCopyInto(out *KubeArmorExternalWorkloadPolicyStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeArmorExternalWorkloadPolicyStatus.
func (in *KubeArmorExternalWorkloadPolicyStatus) DeepCopy() *KubeArmorExternalWorkloadPolicyStatus {
	if in == nil {
		return nil
	}
	out := new(KubeArmorExternalWorkloadPolicyStatus)
	in.DeepCopyInto(out)
	return out
}
