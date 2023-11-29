package main

import (
	"context"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestRunFunctionParsesPrefix(t *testing.T) {
	var ctx context.Context
	req := &fnv1beta1.RunFunctionRequest{
		Input: resource.MustStructJSON(`{
			"apiVersion": "cidrsubnet.fn.crossplane.io/v1beta1",
			"kind": "Input",
			"metadata": {
				"name": "input"
			},
			"prefix": "10.10.300.0/24",
			"newbits": 4,
			"netnum": 1
		}`),
	}

	f := &Function{log: logging.NewNopLogger()}

	rsp, _ := f.RunFunction(ctx, req)

	if len(rsp.Results) < 1 || rsp.Results[0].Severity != fnv1beta1.Severity_SEVERITY_FATAL {
		t.Fatalf("Function must fail if passed an invalid prefix")
	}

}

func TestRunFunctionLimitsNewbits(t *testing.T) {
	var ctx context.Context
	req := &fnv1beta1.RunFunctionRequest{
		Input: resource.MustStructJSON(`{
			"apiVersion": "cidrsubnet.fn.crossplane.io/v1beta1",
			"kind": "Input",
			"metadata": {
				"name": "input"
			},
			"prefix": "10.10.0.0/24",
			"newbits": 9,
			"netnum": 1
		}`),
	}

	f := &Function{log: logging.NewNopLogger()}

	rsp, _ := f.RunFunction(ctx, req)

	if len(rsp.Results) < 1 || rsp.Results[0].Severity != fnv1beta1.Severity_SEVERITY_FATAL {
		t.Fatalf("Function must fail if newbits is greater than the available address space")
	}

}

func TestRunFunctionLimitsNetnum(t *testing.T) {
	var ctx context.Context
	req := &fnv1beta1.RunFunctionRequest{
		Input: resource.MustStructJSON(`{
			"apiVersion": "cidrsubnet.fn.crossplane.io/v1beta1",
			"kind": "Input",
			"metadata": {
				"name": "input"
			},
			"prefix": "10.10.0.0/24",
			"newbits": 4,
			"netnum": 16
		}`),
	}

	f := &Function{log: logging.NewNopLogger()}

	rsp, _ := f.RunFunction(ctx, req)

	if len(rsp.Results) < 1 || rsp.Results[0].Severity != fnv1beta1.Severity_SEVERITY_FATAL {
		t.Fatalf("Function must fail if netnum is greater than the size of newbits")
	}

}

func TestRunFunctionLooksUpPrefix(t *testing.T) {
	var ctx context.Context
	req := &fnv1beta1.RunFunctionRequest{
		Observed: &fnv1beta1.State{
			Resources: map[string]*fnv1beta1.Resource{
				"vpc": {Resource: resource.MustStructJSON(`{
					"apiVersion": "ec2.aws.upbound.io/v1beta1",
					"kind": "VPC",
					"metadata": {
						"name": "vpc"
					},
					"status": {
						"atProvider": {
							"cidrBlock": "10.10.0.0/24",
							"enableDnsHostnames": true,
							"enableDnsSupport": true,
							"region": "us-east-1" 
						}
					}
				}`)},
			},
		},
		Input: resource.MustStructJSON(`{
			"apiVersion": "cidrsubnet.fn.crossplane.io/v1beta1",
			"kind": "Input",
			"metadata": {
				"name": "input"
			},
			"prefix": "${observed.resources[vpc].status.atProvider.cidrBlock}",
			"newbits": 4,
			"netnum": 15
		}`),
	}

	f := &Function{log: logging.NewNopLogger()}

	rsp, _ := f.RunFunction(ctx, req)

	want := &fnv1beta1.RunFunctionResponse{
		Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(60 * time.Second)},
		Context: resource.MustStructJSON(`{
			"input": "10.10.0.240/28"
		}`),
	}

	if diff := cmp.Diff(want, rsp, protocmp.Transform()); diff != "" {
		t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", "", diff)
	}

}

func TestRunFunctionLooksUpNewbits(t *testing.T) {
	var ctx context.Context
	req := &fnv1beta1.RunFunctionRequest{
		Context: resource.MustStructJSON(`{
			"subnet_size": 4
		}`),
		Input: resource.MustStructJSON(`{
			"apiVersion": "cidrsubnet.fn.crossplane.io/v1beta1",
			"kind": "Input",
			"metadata": {
				"name": "input"
			},
			"prefix": "10.10.0.0/24",
			"newbits": "${context.subnet_size}",
			"netnum": 15
		}`),
	}

	f := &Function{log: logging.NewNopLogger()}

	rsp, _ := f.RunFunction(ctx, req)

	want := &fnv1beta1.RunFunctionResponse{
		Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(60 * time.Second)},
		Context: resource.MustStructJSON(`{
			"input": "10.10.0.240/28",
			"subnet_size": 4
		}`),
	}

	if diff := cmp.Diff(want, rsp, protocmp.Transform()); diff != "" {
		t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", "", diff)
	}

}

func TestRunFunctionReturnsSubnet(t *testing.T) {
	var ctx context.Context
	req := &fnv1beta1.RunFunctionRequest{
		Input: resource.MustStructJSON(`{
			"apiVersion": "cidrsubnet.fn.crossplane.io/v1beta1",
			"kind": "Input",
			"metadata": {
				"name": "input"
			},
			"prefix": "10.10.0.0/24",
			"newbits": 4,
			"netnum": 15
		}`),
	}

	f := &Function{log: logging.NewNopLogger()}

	rsp, _ := f.RunFunction(ctx, req)

	want := &fnv1beta1.RunFunctionResponse{
		Meta: &fnv1beta1.ResponseMeta{Ttl: durationpb.New(60 * time.Second)},
		Context: resource.MustStructJSON(`{
			"input": "10.10.0.240/28"
		}`),
	}

	if diff := cmp.Diff(want, rsp, protocmp.Transform()); diff != "" {
		t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", "", diff)
	}

}
