//go:build !ignore_autogenerated

// Code generated by controller-gen. DO NOT EDIT.

package v1beta1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorage) DeepCopyInto(out *SGObjectStorage) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorage.
func (in *SGObjectStorage) DeepCopy() *SGObjectStorage {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SGObjectStorage) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageList) DeepCopyInto(out *SGObjectStorageList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SGObjectStorage, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageList.
func (in *SGObjectStorageList) DeepCopy() *SGObjectStorageList {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SGObjectStorageList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpec) DeepCopyInto(out *SGObjectStorageSpec) {
	*out = *in
	if in.AzureBlob != nil {
		in, out := &in.AzureBlob, &out.AzureBlob
		*out = new(SGObjectStorageSpecAzureBlob)
		(*in).DeepCopyInto(*out)
	}
	if in.Gcs != nil {
		in, out := &in.Gcs, &out.Gcs
		*out = new(SGObjectStorageSpecGcs)
		(*in).DeepCopyInto(*out)
	}
	if in.S3 != nil {
		in, out := &in.S3, &out.S3
		*out = new(SGObjectStorageSpecS3)
		(*in).DeepCopyInto(*out)
	}
	if in.S3Compatible != nil {
		in, out := &in.S3Compatible, &out.S3Compatible
		*out = new(SGObjectStorageSpecS3Compatible)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpec.
func (in *SGObjectStorageSpec) DeepCopy() *SGObjectStorageSpec {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecAzureBlob) DeepCopyInto(out *SGObjectStorageSpecAzureBlob) {
	*out = *in
	in.AzureCredentials.DeepCopyInto(&out.AzureCredentials)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecAzureBlob.
func (in *SGObjectStorageSpecAzureBlob) DeepCopy() *SGObjectStorageSpecAzureBlob {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecAzureBlob)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecAzureBlobAzureCredentials) DeepCopyInto(out *SGObjectStorageSpecAzureBlobAzureCredentials) {
	*out = *in
	if in.SecretKeySelectors != nil {
		in, out := &in.SecretKeySelectors, &out.SecretKeySelectors
		*out = new(SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectors)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecAzureBlobAzureCredentials.
func (in *SGObjectStorageSpecAzureBlobAzureCredentials) DeepCopy() *SGObjectStorageSpecAzureBlobAzureCredentials {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecAzureBlobAzureCredentials)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectors) DeepCopyInto(out *SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectors) {
	*out = *in
	out.AccessKey = in.AccessKey
	out.StorageAccount = in.StorageAccount
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectors.
func (in *SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectors) DeepCopy() *SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectors {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectors)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectorsAccessKey) DeepCopyInto(out *SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectorsAccessKey) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectorsAccessKey.
func (in *SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectorsAccessKey) DeepCopy() *SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectorsAccessKey {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectorsAccessKey)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectorsStorageAccount) DeepCopyInto(out *SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectorsStorageAccount) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectorsStorageAccount.
func (in *SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectorsStorageAccount) DeepCopy() *SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectorsStorageAccount {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecAzureBlobAzureCredentialsSecretKeySelectorsStorageAccount)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecGcs) DeepCopyInto(out *SGObjectStorageSpecGcs) {
	*out = *in
	in.GcpCredentials.DeepCopyInto(&out.GcpCredentials)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecGcs.
func (in *SGObjectStorageSpecGcs) DeepCopy() *SGObjectStorageSpecGcs {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecGcs)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecGcsGcpCredentials) DeepCopyInto(out *SGObjectStorageSpecGcsGcpCredentials) {
	*out = *in
	if in.FetchCredentialsFromMetadataService != nil {
		in, out := &in.FetchCredentialsFromMetadataService, &out.FetchCredentialsFromMetadataService
		*out = new(bool)
		**out = **in
	}
	if in.SecretKeySelectors != nil {
		in, out := &in.SecretKeySelectors, &out.SecretKeySelectors
		*out = new(SGObjectStorageSpecGcsGcpCredentialsSecretKeySelectors)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecGcsGcpCredentials.
func (in *SGObjectStorageSpecGcsGcpCredentials) DeepCopy() *SGObjectStorageSpecGcsGcpCredentials {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecGcsGcpCredentials)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecGcsGcpCredentialsSecretKeySelectors) DeepCopyInto(out *SGObjectStorageSpecGcsGcpCredentialsSecretKeySelectors) {
	*out = *in
	out.ServiceAccountJSON = in.ServiceAccountJSON
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecGcsGcpCredentialsSecretKeySelectors.
func (in *SGObjectStorageSpecGcsGcpCredentialsSecretKeySelectors) DeepCopy() *SGObjectStorageSpecGcsGcpCredentialsSecretKeySelectors {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecGcsGcpCredentialsSecretKeySelectors)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecGcsGcpCredentialsSecretKeySelectorsServiceAccountJSON) DeepCopyInto(out *SGObjectStorageSpecGcsGcpCredentialsSecretKeySelectorsServiceAccountJSON) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecGcsGcpCredentialsSecretKeySelectorsServiceAccountJSON.
func (in *SGObjectStorageSpecGcsGcpCredentialsSecretKeySelectorsServiceAccountJSON) DeepCopy() *SGObjectStorageSpecGcsGcpCredentialsSecretKeySelectorsServiceAccountJSON {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecGcsGcpCredentialsSecretKeySelectorsServiceAccountJSON)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecS3) DeepCopyInto(out *SGObjectStorageSpecS3) {
	*out = *in
	out.AwsCredentials = in.AwsCredentials
	if in.Region != nil {
		in, out := &in.Region, &out.Region
		*out = new(string)
		**out = **in
	}
	if in.StorageClass != nil {
		in, out := &in.StorageClass, &out.StorageClass
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecS3.
func (in *SGObjectStorageSpecS3) DeepCopy() *SGObjectStorageSpecS3 {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecS3)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecS3AwsCredentials) DeepCopyInto(out *SGObjectStorageSpecS3AwsCredentials) {
	*out = *in
	out.SecretKeySelectors = in.SecretKeySelectors
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecS3AwsCredentials.
func (in *SGObjectStorageSpecS3AwsCredentials) DeepCopy() *SGObjectStorageSpecS3AwsCredentials {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecS3AwsCredentials)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecS3AwsCredentialsSecretKeySelectors) DeepCopyInto(out *SGObjectStorageSpecS3AwsCredentialsSecretKeySelectors) {
	*out = *in
	out.AccessKeyId = in.AccessKeyId
	out.SecretAccessKey = in.SecretAccessKey
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecS3AwsCredentialsSecretKeySelectors.
func (in *SGObjectStorageSpecS3AwsCredentialsSecretKeySelectors) DeepCopy() *SGObjectStorageSpecS3AwsCredentialsSecretKeySelectors {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecS3AwsCredentialsSecretKeySelectors)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecS3AwsCredentialsSecretKeySelectorsAccessKeyId) DeepCopyInto(out *SGObjectStorageSpecS3AwsCredentialsSecretKeySelectorsAccessKeyId) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecS3AwsCredentialsSecretKeySelectorsAccessKeyId.
func (in *SGObjectStorageSpecS3AwsCredentialsSecretKeySelectorsAccessKeyId) DeepCopy() *SGObjectStorageSpecS3AwsCredentialsSecretKeySelectorsAccessKeyId {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecS3AwsCredentialsSecretKeySelectorsAccessKeyId)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecS3AwsCredentialsSecretKeySelectorsSecretAccessKey) DeepCopyInto(out *SGObjectStorageSpecS3AwsCredentialsSecretKeySelectorsSecretAccessKey) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecS3AwsCredentialsSecretKeySelectorsSecretAccessKey.
func (in *SGObjectStorageSpecS3AwsCredentialsSecretKeySelectorsSecretAccessKey) DeepCopy() *SGObjectStorageSpecS3AwsCredentialsSecretKeySelectorsSecretAccessKey {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecS3AwsCredentialsSecretKeySelectorsSecretAccessKey)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecS3Compatible) DeepCopyInto(out *SGObjectStorageSpecS3Compatible) {
	*out = *in
	out.AwsCredentials = in.AwsCredentials
	if in.EnablePathStyleAddressing != nil {
		in, out := &in.EnablePathStyleAddressing, &out.EnablePathStyleAddressing
		*out = new(bool)
		**out = **in
	}
	if in.Endpoint != nil {
		in, out := &in.Endpoint, &out.Endpoint
		*out = new(string)
		**out = **in
	}
	if in.Region != nil {
		in, out := &in.Region, &out.Region
		*out = new(string)
		**out = **in
	}
	if in.StorageClass != nil {
		in, out := &in.StorageClass, &out.StorageClass
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecS3Compatible.
func (in *SGObjectStorageSpecS3Compatible) DeepCopy() *SGObjectStorageSpecS3Compatible {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecS3Compatible)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecS3CompatibleAwsCredentials) DeepCopyInto(out *SGObjectStorageSpecS3CompatibleAwsCredentials) {
	*out = *in
	out.SecretKeySelectors = in.SecretKeySelectors
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecS3CompatibleAwsCredentials.
func (in *SGObjectStorageSpecS3CompatibleAwsCredentials) DeepCopy() *SGObjectStorageSpecS3CompatibleAwsCredentials {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecS3CompatibleAwsCredentials)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectors) DeepCopyInto(out *SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectors) {
	*out = *in
	out.AccessKeyId = in.AccessKeyId
	out.SecretAccessKey = in.SecretAccessKey
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectors.
func (in *SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectors) DeepCopy() *SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectors {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectors)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectorsAccessKeyId) DeepCopyInto(out *SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectorsAccessKeyId) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectorsAccessKeyId.
func (in *SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectorsAccessKeyId) DeepCopy() *SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectorsAccessKeyId {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectorsAccessKeyId)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectorsSecretAccessKey) DeepCopyInto(out *SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectorsSecretAccessKey) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectorsSecretAccessKey.
func (in *SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectorsSecretAccessKey) DeepCopy() *SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectorsSecretAccessKey {
	if in == nil {
		return nil
	}
	out := new(SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectorsSecretAccessKey)
	in.DeepCopyInto(out)
	return out
}
